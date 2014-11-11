source /usr/local/go/src/golang-crosscompile/crosscompile.bash
ECHO "Building darwin 386..."
go-darwin-386 build -o ../binaries/force_darwin-386
ECHO "Building darwin AMD64..."
go-darwin-amd64 build -o ../binaries/force_darwin-amd64
ECHO "Building linux 386..."
go-linux-386 build -o ../binaries/force_linux-386
ECHO "Building linux AMD64..."
go-linux-amd64 build -o ../binaries/force_linux-amd64
ECHO "Building linux ARM..."
go-linux-arm build -o ../binaries/force_linux-arm
ECHO "Building windows 386..."
go-windows-386 build -o ../binaries/force_win-386.exe
ECHO "Building windows AMD64..."
go-windows-amd64 build -o ../binaries/force_win-amd64.exe
