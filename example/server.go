package main

import (
	"github.com/andyzhou/tinyrpc"
	"log"
)

//example code for server

//setup relate cb
func cbForNodeDown(addr string) bool {
	log.Printf("cbForNodeDown, addr:%v", addr)
	return true
}
func cbForGenReq(addr string, data []byte) []byte {
	return nil
}
func cbForStreamReq(addr string, data []byte) bool {
	return true
}

func main() {
	//init server
	s := tinyrpc.NewRpcService()

	//set relate cb
	s.SetCBForClientNodeDown(cbForNodeDown)
	s.SetCBForGeneral(cbForGenReq)
	s.SetCBForStream(cbForStreamReq)

	//start service
	s.Start()
}
