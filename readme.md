# Lapis Uno
> External communications code written for CG4002 Computer Engineering Capstone Project

## Summary
Go code for external communications written for a school capstone project.
Dancers have a hardware device which connects over BLE to a NodeJS client on the Dancers laptop. 

NodeJS internal communication code communicates with DataPublisher for IPC over gRPC streams with Protocol Buffers.

DataPublisher publishes sensor readings over MQTT to a secure MQTT broker
The current broker lives at `mqtts.qz.sg` which is a hosted vernemq MQTT broker.
At the time of this repo going public, the broker would have gone offline

To test code, rebuild all relevant files after updating MQTT user, password as well as broker url in DataPublisher as well as DataSubscribers

All DataPublishers publish to their own sensor topics `sensor/<cid>/data`
Singular DataSubscriber subscribes to all dancer topics `sensor/+/data`

Currently only 3 dancers are supported.

This project was a testbed for me to actually learning & write something in Go
as well as to test other technologies like gRPC as well as protocol buffers. They are probably not written with the best practices nor tested and should not be used in production.

## Running the different binaries
---

> Use the binaries build for your platform under releases

### DataPublisher
 
```
Flags:

--mode, string          single or multi , defaults to single, use multi for multi dancers
```

### DataSubscriber
 
```
Flags:

--cid, string           Optional, If passed, mqtt sub will connect using cid as its client. 
                        Pass in something unique to prevent having the same ids as other mqtt clients

--mode, string          single or multi , defaults to single, use multi for multi dancers

--evalclientconn        Optional, Defaults to http://127.0.0.1:10202, not required if running EvalClient on same machine
```
 To run , example, run
```
./DataSubscriber -mode multi
```


### EvalClient
```
Flags:
 
--conn, string          Optional, Eval Server url and port, defaults to 127.0.0.1:12345 

--dashconn. string      Defaults to http://127.0.0.1:3000/api/prediction/ 
                        used to send results of prediction of pos, move & delay to dashboard server

--mode, string          single, multi or standalone , defaults to single, use multi for multi dancers, standalone allows you to test posting http to EvalClient without requiring eval_server.py (not included in this repo)
```

To run, for example
```
./EvalClient-arm64 -mode=multi -conn=<evalserverip:port> -dashconn=<http url to receive predictions>
```
Change `-conn` ip:port to evalserverip and the port eval_server.py is running on (not provided in this repo)
Change `-dashconn` to the http webhook url for your dashboard

There is an alternative client `EvalClientIgnoreDisp-arm64` which ignores if sum of poschanges is non zero

## Misc

`GrpcClient`, `MultiPublisher` as well as `SyncDelay/sub` are used for testing

`GrpcClient` simulates NodeJS sending data to `DataPublisher` over gRPC streams

Some sample NodeJS code is provided in /nodejs for testing purposes and includes a sample on how to subscribe to MQTT topics and writing them to MongoDB
on Mongo Atlas

## Proto format
---
- Changes 25/10/2020 :  
  `clientPos` has been changed to `dancerNo` (used for initial position, never changes once set)   
  `posChange` added (for position changes as a int)
```
posChange values
================
-2 = left x2
-1 = left
 0 = stay
 1 = right
 2 = right x2
```


```proto
syntax = "proto3";
package pb;

option go_package = "github.com/QzSG/lapis-uno/protobuf;pb";

service Sensor {
    // Sends a greeting
    rpc ReadingStream (stream Reading) returns (stream Reply) {}
}

message Reply {
    int32 status = 1;
}

message Reading {
    //Field types are in Go|Python3|C++

    // Indicates if it is the start of a move : *bool|bool|bool
    bool isStartMove = 1;

    // To identify the clients : *string|str|string
    string clientID = 2;

    // Initial assigned dancer number : int32, int, int32
    int32 dancerNo = 3;

    // Position change of client : int32, int, int32
    int32 posChange = 4;
 
    // Accelerometer X Axis value : *float64|float|double
    double accX = 5;
    // Accelerometer Y Axis value : *float64|float|double
    double accY = 6;
    // Accelerometer Z Axis value : *float64|float|double
    double accZ = 7;

    // Gyroscope Roll value : *float64|float|double
    double gyroRoll = 8;
    // Gyroscope Pitch value : *float64|float|double
    double gyroPitch = 9;
    // Gyroscope Yaw value : *float64|float|double
    double gyroYaw = 10;

    // Timestamp in unix nanoseconds : *int64|int/long/int64 
    int64 timeStamp = 11;
}
```

## Disclaimer

All code provided herein is to be treated as non production ready, neither has it undergone rigourious testing.
Use at your own risk. In addition, all provided, urls, credentials, are all no longer in use, and domains may belong to others at the point this repo is made public.