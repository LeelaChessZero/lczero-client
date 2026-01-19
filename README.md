# Compiling

You will need to install Go 1.9 or later.

Then, make sure to set up your GOPATH properly, eg. here is mine:
```
export GOPATH=${HOME}/go:${HOME}/src/lczero-client
```
Here, I've set my system install of go as the first entry, and then the lczero-client directory as the second.

Pre-reqs:
```
# (Bug workaround, using Tilps instead)
# go get -u github.com/notnil/chess
go get -u github.com/Tilps/chess
go get -u github.com/nightlyone/lockfile

```

Pull or download the `master` branch

Then to produce a `lczero-client` executable:
`go build lc0_main.go` for the `lc0` client

If you get
`.\lc0_main.go:1048:5: undefined: chess.GetLibraryVersion`
you have a cached old version of Tilps/chess and need to run the Pre-reqs again.

# Running

First copy the `lc0` executable into the same folder as the `lczero-client` executable.

Then, run!  Username and password are required parameters.
```
./lczero-client --user=myusername --password=mypassword
```

For testing, you can also point the client at a different server:
```
./lczero-client --hostname=http://127.0.0.1:8080 --user=test --password=asdf
```

# Cross-compiling

One of the main reasons I picked go was it's amazing support for cross-compiling.

Pre-reqs:
```
GOOS=windows GOARCH=amd64 go install
GOOS=darwin GOARCH=amd64 go install
GOOS=linux GOARCH=amd64 go install
```

Building the client for each platform:
```
GOOS=windows GOARCH=amd64 go build -o lczero-client.exe
GOOS=darwin GOARCH=amd64 go build -o lczero-client_mac
GOOS=linux GOARCH=amd64 go build -o lczero-client_linux
```


# Go module support 

Dependend go modules were added by executing:

```
go get 'github.com/Tilps/chess@master'    
```

gives something like:
```
go: downloading github.com/Tilps/chess v0.0.0-20200409092358-c35715299813
go: github.com/Tilps/chess master => v0.0.0-20200409092358-c35715299813
```

This version number can then be used in the `go.mod` file

Whenever you want to update the version do the above `go get` step and there will be a new version number generated that you can put in the existing `go.mod` file.

Just use the command `go mod download` to update go's module cache.
building should work with `go build lc0_main.go`

# Docker (GPU)

This repository includes a `Dockerfile` for building an NVIDIA-GPU-enabled image that bundles:
 - `lc0` engine (built from source)
 - `lc0-training-client` (downloaded from GitHub releases)

The container expects a single persistence mount at `/data`:
 - `/data/config` (XDG config home)
 - `/data/cache`  (XDG cache home)

Build:

    docker build -t lc0-training-client:test .

Test (no GPU required):

    docker run --rm lc0-training-client:test --help
    docker run --rm --entrypoint /app/lc0 lc0-training-client:test --help

Test GPU access:

    docker run --rm --gpus all nvidia/cuda:12.9.1-base-ubuntu22.04 nvidia-smi

Run (persist config/cache on host):

    mkdir -p ~/.lc0-training
    docker run --gpus all \
      --user $(id -u):$(id -g) \
      -v ~/.lc0-training:/data \
      lc0-training-client:test \
      --user=USERNAME --password=PASSWORD

Override versions:

    docker build -t lc0-training-client:test \
      --build-arg LC0_VERSION=v0.32.1 \
      --build-arg CLIENT_VERSION=v34 \
      .
