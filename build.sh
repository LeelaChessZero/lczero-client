echo Downloading required dependencies...
echo -ne [0/4]Tilps/chess\\r
go get -u github.com/Tilps/chess
echo -ne [1/4]nightlyone/lockfile\\r
go get -u github.com/nightlyone/lockfile
echo -ne [2/4]jaypipes/ghw\\r
go get -u github.com/jaypipes/ghw
echo -ne [3/4]shettyh/threadpool\\r
go get -u github.com/shettyh/threadpool
echo [4/4]finished Downloading required dependencies
echo building windows...
GOOS=windows GOARCH=amd64 go build
echo finished with errno: $?
echo building linux...
GOOS=linux GOARCH=arm go build
echo finished with errno: $?
