source /usr/local/go/src/golang-crosscompile/crosscompile.bash
ECHO "Building darwin 386..."
go-darwin-386 build -o ../binaries/darwin-386/force
ECHO "Building darwin AMD64..."
go-darwin-amd64 build -o ../binaries/darwin-amd64/force
ECHO "Building linux 386..."
go-linux-386 build -o ../binaries/linux-386/force
ECHO "Building linux AMD64..."
go-linux-amd64 build -o ../binaries/linux-amd64/force
ECHO "Building linux ARM..."
go-linux-arm build -o ../binaries/linux-arm/force
ECHO "Building windows 386..."
go-windows-386 build -o ../binaries/windows-386/force.exe
ECHO "Building windows AMD64..."
go-windows-amd64 build -o ../binaries/windows-amd64/force.exe
aws s3 cp ../binaries s3://force-cli/binaries --recursive
