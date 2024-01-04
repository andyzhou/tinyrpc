package main

import (
	"github.com/andyzhou/tinyrpc"
	"github.com/andyzhou/tinyrpc/proto"
	"log"
	"sync"
	"time"
)

//example code for server

//setup relate cb
func cbForClientNodeDown(addr string) bool {
	log.Printf("cbForNodeDown, addr:%v", addr)
	return true
}
func cbForGenReq(addr string, in *proto.Packet) (*proto.Packet, error) {
	log.Printf("cbForGenReq, addr:%v, in:%v", addr, in)
	return in, nil
}
func cbForStreamReq(addr string, in *proto.Packet) error {
	log.Printf("cbForStreamReq, addr:%v, in:%v", addr, in)
	return nil
}

//send stream data
func sendStreamData(s *tinyrpc.Service) {
	var (
		err error
	)
	//format data
	in := s.GenPacket()
	in.MessageId = 2
	in.Data = []byte("welcome..")

	//send to all
	for {
		err = s.SendStreamData(in)
		if err != nil {
			log.Printf("server.sendStreamData, err:%v\n", err)
		}
		time.Sleep(time.Second * 3)
	}
}

func main() {
	var (
		wg sync.WaitGroup
		m any = nil
	)

	//defer
	defer func() {
		if err := recover(); err != m {
			log.Printf("server panic, err:%v", err)
		}
		wg.Done()
		log.Printf("end server..")
	}()

	//init server
	s := tinyrpc.NewService()

	//set relate cb
	s.SetCBForClientNodeDown(cbForClientNodeDown)
	s.SetCBForGeneral(cbForGenReq)
	s.SetCBForStream(cbForStreamReq)

	//start service
	wg.Add(1)
	log.Printf("start server on port %v..", s.GetPort())
	s.Start()
	go sendStreamData(s)
	wg.Wait()
}
