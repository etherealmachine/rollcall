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
