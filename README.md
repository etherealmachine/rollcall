# rollcall - Rolling Call Market Service

Get the code:
```
git clone https://github.com/etherealmachine/rollcall.git
```

Run the server:
```
cd rollcall/server
go run rollcall_server.go
```

Run the example client:
```
cd rollcall/client
go run client.go
```

Regenerate the proto:
```
protoc -I ./proto/ ./proto/rollcall.proto --go_out=plugins=grpc:proto/
```

### Overview
Rollcall implements a rolling call market service. The market continuously accepts limit orders to buy or sell quantities of some asset at a specified price. Every N seconds (a "call"), the market evaluates the current order queues and determines what (if any) orders will transact. Interested listeners can register themselves and the market will broadcasts transactions to them.

### Technical Details
The service is implemented in Go ([golang](https://golang.org/)) using [gRPC](http://www.grpc.io/). An example client is provided in Go, but clients can be written in any language supported by gRPC (currently C, C++, Java, Go, Node.js, Python, Ruby, Objective-C, PHP and C#). The service listens on a single port (currently hardcoded to 50051). Interested clients can refer to the proto definition in [rollcall.proto](https://github.com/etherealmachine/rollcall/blob/master/proto/rollcall.proto) for details.
