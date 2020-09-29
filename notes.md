# Notes

```bash
#Get average elapsed time
awk '{ total += $2; count++ } END { print total/count "ms"}' output.txt 
```

```bash
#Compile protobufs using protoc
protoc --go_out=paths=source_relative:. protobuf/*proto
```

Packages needed : Install `protoc-gen-go` and run `go get`

### MQTT Client Conn

1. No client certs required (auth using mqtt userpass auth)
1. No CA cert required for server cert as server cert is LE signed