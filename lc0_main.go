// A new client to work with the lc0 binary.
//
//
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"client"

	"github.com/Tilps/chess"
	"github.com/nightlyone/lockfile"
)

var (
	startTime  time.Time
	totalGames int
	pendingNextGame *client.NextGameResponse

	hostname = flag.String("hostname", "http://api.lczero.org", "Address of the server")
	user     = flag.String("user", "", "Username")
	password = flag.String("password", "", "Password")
//	gpu      = flag.Int("gpu", -1, "ID of the OpenCL device to use (-1 for default, or no GPU)")
	debug    = flag.Bool("debug", false, "Enable debug mode to see verbose output and save logs")
	lc0Args  = flag.String("lc0args", "", "")
	backopts = flag.String("backend-opts", "",
		`Options for the lc0 mux. backend. Example: --backend-opts="cudnn(gpu=1)"`)
	parallel = flag.Int("parallelism", -1, "Number of games to play in parallel (-1 for default)")
	useTestServer = flag.Bool("use-test-server", false, "Set host name to test server.")
)

// Settings holds username and password.
type Settings struct {
	User string
	Pass string
}

/*
	Reads the user and password from a config file and returns empty strings if anything went wrong.
	If the config file does not exists, it prompts the user for a username and password and creates the config file.
*/
func readSettings(path string) (string, string) {
	settings := Settings{}
	file, err := os.Open(path)
	if err != nil {
		// File was not found
		fmt.Printf("Please enter your username and password, an account will be automatically created.\n")
		fmt.Printf("Note that this password will be stored in plain text, so avoid a password that is\n")
		fmt.Printf("also used for sensitive applications. It also cannot be recovered.\n")
		fmt.Printf("Enter username : ")
		fmt.Scanf("%s\n", &settings.User)
		fmt.Printf("Enter password : ")
		fmt.Scanf("%s\n", &settings.Pass)
		jsonSettings, err := json.Marshal(settings)
		if err != nil {
			log.Fatal("Cannot encode settings to JSON ", err)
			return "", ""
		}
		settingsFile, err := os.Create(path)
		defer settingsFile.Close()
		if err != nil {
			log.Fatal("Could not create output file ", err)
			return "", ""
		}
		settingsFile.Write(jsonSettings)
		return settings.User, settings.Pass
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&settings)
	if err != nil {
		log.Fatal("Error decoding JSON ", err)
		return "", ""
	}
	return settings.User, settings.Pass
}

func getExtraParams() map[string]string {
	return map[string]string{
		"user":     *user,
		"password": *password,
		"version":  "16",
	}
}

func uploadGame(httpClient *http.Client, path string, pgn string,
	nextGame client.NextGameResponse, version string, fp_threshold float64) error {

	var retryCount uint32

	for {
		retryCount++
		if retryCount > 3 {
			return errors.New("UploadGame failed: Too many retries")
		}

		extraParams := getExtraParams()
		extraParams["training_id"] = strconv.Itoa(int(nextGame.TrainingId))
		extraParams["network_id"] = strconv.Itoa(int(nextGame.NetworkId))
		extraParams["pgn"] = pgn
		extraParams["engineVersion"] = version
		if fp_threshold >= 0.0 {
			extraParams["fp_threshold"] = strconv.FormatFloat(fp_threshold, 'E', -1, 64)
		}
		request, err := client.BuildUploadRequest(*hostname+"/upload_game", extraParams, "file", path)
		if err != nil {
			log.Printf("BUR: %v", err)
			return err
		}
		resp, err := httpClient.Do(request)
		if err != nil {
			log.Printf("http.Do: %v", err)
			return err
		}
		body := &bytes.Buffer{}
		_, err = body.ReadFrom(resp.Body)
		if err != nil {
			log.Print(err)
			log.Print("Error uploading, retrying...")
			time.Sleep(time.Second * (2 << retryCount))
			continue
		}
		resp.Body.Close()
		break
	}

	totalGames++
	log.Printf("Completed %d games in %s time", totalGames, time.Since(startTime))

	err := os.Remove(path)
	if err != nil {
		log.Printf("Failed to remove training file: %v", err)
	}

	return nil
}

type gameInfo struct {
	pgn   string
	fname string
	// If >= 0, this is the value which if resign threshold was set 
	// higher a false positive would have occurred if the game had been
	// played with resign.
	fp_threshold float64
}

type cmdWrapper struct {
	Cmd      *exec.Cmd
	Pgn      string
	Input    io.WriteCloser
	BestMove chan string
	gi       chan gameInfo
	Version  string
}

func (c *cmdWrapper) openInput() {
	var err error
	c.Input, err = c.Cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
}

func convertMovesToPGN(moves []string) string {
	game := chess.NewGame(chess.UseNotation(chess.LongAlgebraicNotation{}))
	for _, m := range moves {
		err := game.MoveStr(m)
		if err != nil {
			log.Fatalf("movstr: %v", err)
		}
	}
	game2 := chess.NewGame()
	b, err := game.MarshalText()
	if err != nil {
		log.Fatalf("MarshalText failed: %v", err)
	}
	game2.UnmarshalText(b)
	return game2.String()
}

func createCmdWrapper() *cmdWrapper {
	c := &cmdWrapper{
		gi:       make(chan gameInfo),
		BestMove: make(chan string),
		Version: "v0.10.0",
	}
	return c
}

func (c *cmdWrapper) launch(networkPath string, args []string, input bool) {
	dir, _ := os.Getwd()
	c.Cmd = exec.Command(path.Join(dir, "lc0"))
	// Add the "selfplay" or "uci" part first
	mode := args[0]
	c.Cmd.Args = append(c.Cmd.Args, mode)
	args = args[1:]
	if mode != "selfplay" {
		c.Cmd.Args = append(c.Cmd.Args, "--backend=multiplexing")
	}
	if *lc0Args != "" {
		log.Println("WARNING: Option --lc0args is for testing, not production use!")
		log.SetPrefix("TESTING: ")
		parts := strings.Split(*lc0Args, " ")
		c.Cmd.Args = append(c.Cmd.Args, parts...)
	}
	if *backopts != "" {
		// Check agains small token blacklist, currently only "random"
		tokens := regexp.MustCompile("[,=().0-9]").Split(*backopts, -1)
		for _, token := range tokens {
			switch token {
				case "random":
				log.Fatalf("Not accepted in --backend-opts: %s", token)
			}
		}
		c.Cmd.Args = append(c.Cmd.Args, fmt.Sprintf("--backend-opts=%s", *backopts))
	}
	if *parallel > 0 && mode == "selfplay" {
		c.Cmd.Args = append(c.Cmd.Args, fmt.Sprintf("--parallelism=%v", *parallel))
	}
	c.Cmd.Args = append(c.Cmd.Args, args...)
	c.Cmd.Args = append(c.Cmd.Args, fmt.Sprintf("--weights=%s", networkPath))
	if !*debug {
		//		c.Cmd.Args = append(c.Cmd.Args, "--quiet")
		fmt.Println("lc0 is never quiet.")
	}
	fmt.Printf("Args: %v\n", c.Cmd.Args)

	stdout, err := c.Cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	c.Cmd.Stderr = os.Stdout

	// If the game wasn't played with resign, and the engine supports it,
	// this will be populated by the resign_report before the gameready
	// with the value which the resign threshold should be kept below to
	// avoid a false positive.
	last_fp_threshold := -1.0
	go func() {
		defer close(c.BestMove)
		defer close(c.gi)
		stdoutScanner := bufio.NewScanner(stdout)
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			//			fmt.Printf("lc0: %s\n", line)
			switch {
			case strings.HasPrefix(line, "resign_report "):
				args := strings.Split(line, " ")
				fp_threshold_idx := -1
				for idx, arg := range args {
					if arg == "fp_threshold" {
						fp_threshold_idx = idx+1
					}
				}
				if fp_threshold_idx >= 0 {
					last_fp_threshold, err = strconv.ParseFloat(args[fp_threshold_idx], 64)
					if err != nil {
						log.Printf("Malformed resign_report: %q", line)
						last_fp_threshold = -1.0
					}
				}
				fmt.Println(line)
			case strings.HasPrefix(line, "gameready "):
				// filename is between "trainingfile" and "gameid"
				idx1 := strings.Index(line, "trainingfile")
				idx2 := strings.LastIndex(line, "gameid")
				idx3 := strings.LastIndex(line, "moves")
				if idx1 < 0 || idx2 < 0 || idx3 < 0 {
					log.Printf("Malformed gameready: %q", line)
					break
				}
				file := line[idx1+13:idx2-1]
				pgn := convertMovesToPGN(strings.Split(line[idx3+6:len(line)]," "))
				fmt.Printf("PGN: %s\n", pgn)
				c.gi <- gameInfo{pgn: pgn, fname: file, fp_threshold: last_fp_threshold}
				last_fp_threshold = -1.0
			case strings.HasPrefix(line, "bestmove "):
				//				fmt.Println(line)
				c.BestMove <- strings.Split(line, " ")[1]
			case strings.HasPrefix(line, "id name The Lc0 chess engine. "):
				c.Version = strings.Split(line, " ")[6]
				fmt.Println(line)
			case strings.HasPrefix(line, "info"):
				break
				fallthrough
			default:
				fmt.Println(line)
			}
		}
	}()

	if input {
		c.openInput()
	}

	err = c.Cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func playMatch(baselinePath string, candidatePath string, params []string, flip bool) (int, string, string, error) {
	baseline := createCmdWrapper()
	params = append([]string{"uci"}, params...)
	log.Println("launching 1")
	baseline.launch(baselinePath, params, true)
	defer baseline.Input.Close()
	defer func() {
		log.Println("Waiting for baseline to exit.")
		baseline.Cmd.Process.Kill()
		baseline.Cmd.Wait()
	}()

	candidate := createCmdWrapper()
	log.Println("launching 2")
	candidate.launch(candidatePath, params, true)
	defer candidate.Input.Close()
	defer func() {
		log.Println("Waiting for candidate to exit.")
		candidate.Cmd.Process.Kill()
		candidate.Cmd.Wait()
	}()

	p1 := candidate
	p2 := baseline

	if flip {
		p2, p1 = p1, p2
	}

	log.Println("writing uci")
	io.WriteString(baseline.Input, "uci\n")
	io.WriteString(candidate.Input, "uci\n")

	// Play a game using UCI
	var result int
	game := chess.NewGame(chess.UseNotation(chess.LongAlgebraicNotation{}))
	moveHistory := ""
	turn := 0
	for {
		if turn >= 450 || game.Outcome() != chess.NoOutcome || len(game.EligibleDraws()) > 1 {
			if game.Outcome() == chess.WhiteWon {
				result = 1
			} else if game.Outcome() == chess.BlackWon {
				result = -1
			} else {
				result = 0
			}

			// Always report the result relative to the candidate engine
			// (which defaults to white, unless flip = true)
			if flip {
				result = -result
			}
			log.Printf("result: %d\n", result)
			break
		}

		var p *cmdWrapper
		if game.Position().Turn() == chess.White {
			p = p1
		} else {
			p = p2
		}
		io.WriteString(p.Input, "position startpos"+moveHistory+"\n")
		io.WriteString(p.Input, "go nodes 800\n")

		select {
		case bestMove, ok := <-p.BestMove:
			if !ok {
				log.Println("engine failed")
				p.BestMove = nil
				break
			}
			err := game.MoveStr(bestMove)
			if err != nil {
				log.Println("Error decoding: " + bestMove + " for game:\n" + game.String())
				return 0, "", "", err
			}
			if len(moveHistory) == 0 {
				moveHistory = " moves"
			}
			moveHistory += " " + bestMove
			turn++
		case <-time.After(60 * time.Second):
			log.Println("Bestmove has timed out, aborting match")
			return 0, "", "", errors.New("timeout")
		}
	}

	chess.UseNotation(chess.AlgebraicNotation{})(game)
	fmt.Printf("PGN: %s\n", game.String())
	return result, game.String(), candidate.Version, nil
}

func train(httpClient *http.Client, ngr client.NextGameResponse,
	networkPath string, count int, params []string, doneCh chan bool) error {
	// pid is intended for use in multi-threaded training
	pid := os.Getpid()

	dir, _ := os.Getwd()
	if *debug {
		logsDir := path.Join(dir, fmt.Sprintf("logs-%v", pid))
		os.MkdirAll(logsDir, os.ModePerm)
		logfile := path.Join(logsDir, fmt.Sprintf("%s.log", time.Now().Format("20060102150405")))
		params = append(params, "-l"+logfile)
	}

	// lc0 needs selfplay first in the argument list.
	params = append([]string{"selfplay"}, params...)
	params = append(params, "--training=true")
	c := createCmdWrapper()
	c.launch(networkPath, params /* input= */, false)
	trainDir := ""
	defer func() {
		// Remove the training dir when we're done training.
		if trainDir != "" {
			log.Printf("Removing traindir: %s", trainDir)
			err := os.RemoveAll(trainDir)
			if err != nil {
				log.Printf("Error removing train dir: %v", err)
			}
		}
	}()
	wg := &sync.WaitGroup{}
	numGames := 1
	progressOrKill := false
	for done := false; !done; {
		select {
		case <-doneCh:
			done = true
			progressOrKill = true
			log.Println("Received message to end training, killing lc0")
			c.Cmd.Process.Kill()
		case _, ok := <-c.BestMove:
			// Just swallow the best moves, only needed for match play.
			if !ok {
				log.Printf("BestMove channel closed unexpectedly, exiting train loop")
				break
			}
		case gi, ok := <-c.gi:
			if !ok {
				log.Printf("GameInfo channel closed, exiting train loop")
				done = true
				break
			}
			fmt.Printf("Uploading game: %d\n", numGames)
			numGames++
			progressOrKill = true
			trainDir = path.Dir(gi.fname)
			log.Printf("trainDir=%s", trainDir)
			wg.Add(1)
			go func() {
				uploadGame(httpClient, gi.fname, gi.pgn, ngr, c.Version, gi.fp_threshold)
				wg.Done()
			}()
		}
	}

	log.Println("Waiting for lc0 to stop")
	err := c.Cmd.Wait()
	if err != nil {
		fmt.Printf("lc0 exited with: %v", err)
	}
	log.Println("lc0 stopped")

	log.Println("Waiting for uploads to complete")
	wg.Wait()
	if !progressOrKill {
		return errors.New("Client self-exited without producing any games.")
	}
	return nil
}

func checkValidNetwork(sha string) (string, error) {
	// Sha already exists?
	path := filepath.Join("networks", sha)
	stat, err := os.Stat(path)
	if err == nil {
		if stat.Size() != 0 {
			file, _ := os.Open(path)
			reader, err := gzip.NewReader(file)
			if err == nil {
				_, err = ioutil.ReadAll(reader)
			}
			file.Close()
			if err != nil {
				fmt.Printf("Deleting invalid network...\n")
				os.Remove(path)
				return path, err
			} else {
				return path, nil
			}
		}
	}
	return path, err
}

func removeAllExcept(dir string, sha string) (error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Name() == sha {
			continue
		}
		fmt.Printf("Removing %v\n", file.Name())
		err := os.RemoveAll(filepath.Join(dir, file.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func acquireLock(sha string) (lockfile.Lockfile, error) {
	lockpath, _ := filepath.Abs(filepath.Join("networks", sha + ".lck"))
	lock, err := lockfile.New(lockpath)
	if err != nil {
		// Unknown error. Exit.
		log.Fatalf("Cannot init lockfile: %v", err)
	}
	// Attempt to acquire lock
	err = lock.TryLock()
	return lock, err
}

func getNetwork(httpClient *http.Client, sha string, clearOld bool) (string, error) {
	
	os.MkdirAll("networks", os.ModePerm)

	for {
		// Loop until a valid network is found
		path, err := checkValidNetwork(sha)
		if err == nil {
			// There is already a valid network. Use it.

			if clearOld {
				err := removeAllExcept("networks", sha)
				if err != nil {
					log.Printf("Failed to remove old network(s): %v", err)
				}
			}

			return path, nil
		}

		// Otherwise, let's download it
		lock, err := acquireLock(sha)
		if err == nil {
			// Lockfile acquired, download it
			fmt.Printf("Downloading network...\n")
			err = client.DownloadNetwork(httpClient, *hostname, path, sha)
			if err != nil {
				log.Printf("Network download failed: %v", err)
			}
			lock.Unlock()
		} else if err == lockfile.ErrBusy {
			log.Println("Download initiated by other client. Sleeping for 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			log.Fatalf("Unable to lock: %v", err);
		}
	}
}

func nextGame(httpClient *http.Client, count int) error {
	var nextGame client.NextGameResponse
	var err error
	if pendingNextGame != nil {
		nextGame = *pendingNextGame
		pendingNextGame = nil
		err = nil
	} else {
		nextGame, err = client.NextGame(httpClient, *hostname, getExtraParams())
		if err != nil {
			return err
		}
	}
	var serverParams []string
	err = json.Unmarshal([]byte(nextGame.Params), &serverParams)
	if err != nil {
		return err
	}
	log.Printf("serverParams: %s", serverParams)

	if nextGame.Type == "match" {
		log.Println("Starting match")
		networkPath, err := getNetwork(httpClient, nextGame.Sha, false)
		if err != nil {
			return err
		}
		candidatePath, err := getNetwork(httpClient, nextGame.CandidateSha, false)
		if err != nil {
			return err
		}
		log.Println("Starting match")
		result, pgn, version, err := playMatch(networkPath, candidatePath, serverParams, nextGame.Flip)
		if err != nil {
			log.Printf("playMatch: %v", err)
			return err
		}
		extraParams := getExtraParams()
		extraParams["engineVersion"] = version
		log.Println("uploading match result")
		go client.UploadMatchResult(httpClient, *hostname, nextGame.MatchGameId, result, pgn, extraParams)
		return nil
	}

	if nextGame.Type == "train" {
		networkPath, err := getNetwork(httpClient, nextGame.Sha, true)
		if err != nil {
			return err
		}
		doneCh := make(chan bool)
		go func() {
			errCount := 0
			for {
				time.Sleep(60 * time.Second)
				if nextGame.Type == "Done" {
					return
				}
				ng, err := client.NextGame(httpClient, *hostname, getExtraParams())
				if err != nil {
					fmt.Printf("Error talking to server: %v\n", err)
					errCount++
					if errCount < 10 {
						continue
					}
					doneCh <- true
					close(doneCh)
					return
				}
				if ng.Type != nextGame.Type || ng.Sha != nextGame.Sha {
					pendingNextGame = &ng
					doneCh <- true
					close(doneCh)
					return
				}
				errCount = 0
			}
		}()
		err = train(httpClient, nextGame, networkPath, count, serverParams, doneCh)
		// Ensure the anonymous function stops retrying.
		nextGame.Type = "Done"
		if err != nil {
			return err
		}
		return nil
	}

	return errors.New("Unknown game type: " + nextGame.Type)
}

// Check if PGN may contain "e.p." to verify that the chess package is recent
func testEP() {
	game := chess.NewGame(chess.UseNotation(chess.AlgebraicNotation{}))
	game.MoveStr("a4")
	game.MoveStr("c5")
	game.MoveStr("a5")
	game.MoveStr("b5")
	game.MoveStr("axb6")

	if strings.Contains(game.String(),"e.p.") {
		log.Fatal("You need a more recent version of package github.com/Tilps/chess")
	}
}

func hideLc0argsFlag() {
	shown := new(flag.FlagSet)
	flag.VisitAll(func(f *flag.Flag) {
		if (f.Name != "lc0args") {
			shown.Var(f.Value, f.Name, f.Usage)
		}
	})
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		shown.PrintDefaults()
	}
}

func main() {
	testEP()

	hideLc0argsFlag()
	flag.Parse()

	if *useTestServer {
		*hostname = "http://testserver.lczero.org"
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if len(*user) == 0 || len(*password) == 0 {
		*user, *password = readSettings("settings.json")
	}

	if len(*user) == 0 {
		log.Fatal("You must specify a username")
	}
	if len(*password) == 0 {
		log.Fatal("You must specify a non-empty password")
	}

	httpClient := &http.Client{}
	startTime = time.Now()
	for i := 0; ; i++ {
		err := nextGame(httpClient, i)
		if err != nil {
			log.Print(err)
			log.Print("Sleeping for 30 seconds...")
			time.Sleep(30 * time.Second)
			continue
		}
	}
}
