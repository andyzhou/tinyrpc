# about
This is a gen rpc library, support service and client mode, base on grpc, use default port 7100.

# 3rd depend
- need go v1.8 or high version.

# future
- support gen and stream mode
- dev seldom code for user side

# about core code
- service.go, the api for server side
- client.go, the api for client side

# proto generate
cd proto
protoc --go_out=plugins=grpc:. *.proto

# example
- see `example` dir

# testing
```
cd testing
go test -v
go test -v -run="GenRpc"

go test -bench=.
go test -bench=GenRpc
go test -bench=GenRpc -benchmem -benchtime=20s

```
