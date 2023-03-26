package main

import (
	"github.com/andyzhou/tinyrpc"
	"log"
	"sync"
	"time"
)

//example code for server

//setup relate cb
func cbForNodeDown(addr string) bool {
	log.Printf("cbForNodeDown, addr:%v", addr)
	return true
}
func cbForGenReq(addr string, data []byte) ([]byte, error) {
	log.Printf("cbForGenReq, addr:%v, data:%v", addr, data)
	return data, nil
}
func cbForStreamReq(addr string, data []byte) bool {
	log.Printf("cbForStreamReq, addr:%v, data:%v", addr, data)
	return true
}

//send stream data
func sendStreamData(s *tinyrpc.RpcService) {
	//format data
	in := s.GenPacket()
	in.MessageId = 2
	in.Data = []byte("welcome..")

	//send to all
	for {
		err := s.SendStreamToAll(in)
		log.Printf("server.sendStreamData, err:%v", err)
		time.Sleep(time.Second * 3)
	}
}

func main() {
	var (
		wg sync.WaitGroup
	)

	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("server panic, err:%v", err)
		}
		wg.Done()
		log.Printf("end server..")
	}()

	//init server
	s := tinyrpc.NewRpcService()

	//set relate cb
	s.SetCBForClientNodeDown(cbForNodeDown)
	s.SetCBForGeneral(cbForGenReq)
	s.SetCBForStream(cbForStreamReq)

	//start service
	wg.Add(1)
	log.Printf("start server on port %v..", s.GetPort())
	s.Start()
	go sendStreamData(s)
	wg.Wait()
}
