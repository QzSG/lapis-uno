# Readme 

## Running the different binaries
---

### DataSubscriber
 
```
Flags:

--cid, string       Optional, If passed, mqtt sub will connect using cid as its client. 
                    Pass in something unique to prevent having the same ids as other mqtt clients

--mode              single or multi , defaults to single, use multi for multi dancers

--evalclientconn    Optional, Defaults to http://127.0.0.1:10202, not required if running EvalClient on same machine
```

### EvalClient
```
Flags:
 
--conn, string      Optional, Eval Server url and port, defaults to 127.0.0.1:12345 

--dashconn          Defaults to http://127.0.0.1:3000/api/prediction/ 
                    used to send results of prediction of pos, move & delay to dashboard server

--mode              single or multi , defaults to single, use multi for multi dancers
```

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