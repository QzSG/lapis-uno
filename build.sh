echo "Starting build"
#linux amd64
echo "Building for linux amd64"
go build -o build/EvalClient-linux-amd64 cmd/EvalClient/main.go
go build -o build/EvalClientIgnoreDisp-linux-amd64 cmd/EvalClientIgnoreDisp/main.go
go build -o build/DataPublisher-linux-amd64 cmd/DataPublisher/main.go 
go build -o build/DataSubscriber-linux-amd64 cmd/DataSubscriber/main.go
#linux arm64
echo "Building for linux arm64"
env GOARCH=arm64 GOOS=linux go build -o build/EvalClient-arm64 cmd/EvalClient/main.go
env GOARCH=arm64 GOOS=linux go build -o build/EvalClientIgnoreDisp-arm64 cmd/EvalClientIgnoreDisp/main.go
env GOARCH=arm64 GOOS=linux go build -o build/DataSubscriber-arm64 cmd/DataSubscriber/main.go
env GOARCH=arm64 GOOS=linux go build -o build/DataPublisher-arm64 cmd/DataPublisher/main.go 
#pi arm7
echo "Building for rpi arm7"
env GOOS=linux GOARCH=arm GOARM=7 go build -o build/DataPublisher-pi-arm7 cmd/DataPublisher/main.go 
#darwin amd64
echo "Building for darwin amd64"
env GOOS=darwin GOARCH=amd64 go build -o build/DataSubscriber-darwin-amd64 cmd/DataSubscriber/main.go