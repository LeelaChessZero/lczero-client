package client

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildUploadRequest(t *testing.T) {

	// ARRANGE create a valid file
	// that can be used for upload
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	println(dir)
	filetowrite := filepath.Join(dir, "testresult.pgn")
	println(filetowrite)
	file, err := os.OpenFile(filetowrite, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	file.Write([]byte(filecontents))
	file.Close()

	var params map[string]string

	// Act
	httpreq, err := BuildUploadRequest("http://some.uri", params, "file", filetowrite)

	// Assert
	assert.Nil(t, err)
	assert.NotNil(t, httpreq)
	assert.True(t, httpreq.Method == "POST")
	assert.True(t, httpreq.URL.Host == "some.uri")
	assert.True(t, httpreq.ContentLength == 2335) // length of the fileconstant

}

const filecontents = `[Event "30netstest"]
[Site "HaceTourney"]
[Date "2018.12.28\"]
[Round "5"]
[White "lc0-v19-Net-32270"]
[Black "lc0-v19-Net-32300"]
[Result "1/2-1/2"]
[ECO "E00"]
[Opening "Catalan Opening"]
[PlyCount "88"]
[TimeControl "10/move"]

1. d4 {book} Nf6 {book} 2. c4 {book} e6 {book} 3. g3 {+0.50/9 7.2s}
Bb4+ {-0.48/10 9.6s} 4. Bd2 {+0.63/9 5.5s} Be7 {-0.47/10 4.2s}
5. Nf3 {+0.57/9 9.5s} d5 {-0.40/9 5.6s} 6. Bg2 {+0.56/9 7.7s} O-O {-0.37/9 4.6s}
7. Qc2 {+0.53/9 8.0s} c6 {-0.33/9 4.0s} 8. O-O {+0.49/9 8.1s}
Nbd7 {-0.27/9 4.2s} 9. Bf4 {+0.48/9 6.7s} b6 {-0.26/9 4.5s}
10. cxd5 {+0.46/9 3.3s} cxd5 {-0.30/9 3.0s} 11. Rc1 {+0.43/9 8.1s}
Bb7 {-0.26/8 4.4s} 12. Qb3 {+0.33/8 8.6s} Rc8 {-0.15/8 4.8s}
13. Rxc8 {+0.25/8 9.1s} Qxc8 {-0.11/8 4.0s} 14. Nc3 {+0.25/8 3.2s}
a6 {-0.07/8 4.1s} 15. Rc1 {+0.20/8 8.7s} Qa8 {-0.13/8 7.1s}
16. Na4 {+0.20/9 4.2s} b5 {-0.09/10 3.6s} 17. Nc5 {+0.17/9 2.7s}
Rc8 {-0.05/8 3.3s} 18. Nxd7 {+0.16/8 8.2s} Nxd7 {-0.05/8 4.7s}
19. Qd1 {+0.16/7 5.6s} b4 {-0.04/7 9.6s} 20. Bf1 {+0.24/6 7.4s}
h6 {-0.06/7 8.4s} 21. h4 {+0.22/7 8.6s} a5 {-0.10/8 5.3s}
22. Rxc8+ {+0.17/8 7.5s} Qxc8 {+0.08/10 3.9s} 23. Qa4 {+0.14/10 4.8s}
Nb6 {+0.18/12 4.1s} 24. Qxa5 {-0.01/11 4.2s} Nc4 {+0.20/11 2.7s}
25. Qa7 {+0.03/10 2.8s} Nxb2 {+0.18/11 2.2s} 26. Ne5 {+0.04/9 2.5s}
Qa8 {+0.12/10 9.2s} 27. Qb6 {+0.07/9 8.7s} Qc8 {+0.16/10 4.5s}
28. h5 {+0.10/10 5.3s} Nc4 {+0.12/10 3.4s} 29. Qa7 {+0.10/10 3.1s}
Nxe5 {+0.26/11 5.4s} 30. Bxe5 {-0.04/10 3.9s} Qa8 {+0.27/11 2.6s}
31. Qxa8+ {-0.11/10 5.4s} Bxa8 {+0.16/10 2.6s} 32. e3 {+0.01/9 2.7s}
Bc6 {+0.11/9 2.6s} 33. Bd3 {+0.04/8 2.9s} Ba4 {+0.08/8 6.0s}
34. f3 {+0.12/7 4.3s} Bd1 {-0.02/7 9.2s} 35. Kf2 {+0.13/7 6.2s} f5 {0.00/7 8.9s}
36. Bc7 {+0.12/7 8.3s} Kf7 {+0.03/7 7.2s} 37. Bf4 {+0.14/6 9.2s}
Bd8 {+0.01/7 9.8s} 38. Bd6 {+0.11/7 8.8s} Be7 {-0.04/7 6.2s}
39. Bc7 {+0.11/7 8.7s} Kf8 {0.00/7 9.8s} 40. Be5 {+0.12/7 9.1s}
Kf7 {-0.02/7 7.3s} 41. Bc7 {+0.08/7 9.0s} Bf8 {-0.02/7 8.3s}
42. Be5 {+0.14/6 8.5s} Be7 {0.00/6 6.1s} 43. Bf4 {+0.08/6 9.7s}
Bf8 {-0.05/7 9.3s} 44. Be5 {+0.11/6 9.1s}
Be7 {0.00/2 5.7s, Draw by 3-fold repetition} 1/2-1/2`
