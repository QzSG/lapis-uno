# Notes

```bash
#Get average elapsed time
awk '{ total += $2; count++ } END { print total/count "ms"}' output.txt 
```

```bash
#Compile protobufs using protoc
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protobuf/*proto
```

Packages needed : `protoc` latest binary 3.13.0 from github, 
```bash
export GO111MODULE=on  # Enable module mode
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.0
go get -u google.golang.org/grpc
```

### Protobuf definition

Refer to `reading.proto` in protobuf folder for datatypes

### MQTT Client Conn

1. No client certs required (auth using mqtt userpass auth)
1. No CA cert required for server cert as server cert is LE signed

`DataPublisher` sends data received from grpc client calls (overwrites timestamp)
`DataSubscriber` subs to topic `sensor/+/data`
`MultiSubscriber` simulates 3 clients sending at 50Hz for 1 second at the same time (goroutine)
`SyncDelay` *pub* can be used to test delay (time between fastest & slowest dancer), *sub* prints out sync delay once 3 start packets received, indeterminate since calc runs in a goroutine, change to output to a message channel if needed

// to run ntpclient on its own change its package to main then do : go run NTPClient.go
`NTPClient` sends one single ntp req to sg.pool.ntp.org and returns offset

All clients uses monotonic clock elapsed since initial clock
Time returned (unix nanoseconds) = init_wall_clock + monotonic time elapsed + offset

### Steps

1. Run DataPublisher.go first (starts grpc server and mqtt client)
2. Start nodejs comms code

