package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/andyzhou/tinyrpc"
	"github.com/andyzhou/tinyrpc/proto"
)

//example code for server

//global variable
var (
	s *tinyrpc.Service
)

//setup relate cb
func cbForClientNodeUp(addr string) error {
	log.Printf("cbForNodeUp, addr:%v", addr)
	return nil
}

func cbForClientNodeDown(addr string) error {
	log.Printf("cbForNodeDown, addr:%v", addr)
	return nil
}

func cbForGenReq(addr string, in *proto.Packet) (*proto.Packet, error) {
	log.Printf("cbForGenReq, addr:%v, in:%v", addr, in)
	return in, nil
}
func cbForStreamReq(addr string, in *proto.Packet) error {
	log.Printf("cbForStreamReq, addr:%v, in:%v", addr, in)

	//send reply to client
	//format data
	reply := s.GenPacket()
	reply.MessageId = 2
	reply.Data = []byte(fmt.Sprintf("reply to %v", addr))
	s.SendStreamData(reply, addr)
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

//cpu pprof
func startCpuProfile()  {
	//create cpu pprof file
	cpuProfFile, err := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(any(err))
	}

	//start cpu profile
	err = pprof.StartCPUProfile(cpuProfFile)
	if err != nil {
		panic(any(err))
	}
}

//memory pprof
func startMemoryProfile()  {
	//create memory pprof file
	memProfFile, err := os.OpenFile("mem.pprof", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(any(err))
	}

	//start cpu profile
	err = pprof.WriteHeapProfile(memProfFile)
	if err != nil {
		panic(any(err))
	}
}

func stopProfile() {
	pprof.StopCPUProfile()
}

//start profile
func startProfile() {
	port := 6060
	addr := fmt.Sprintf(":%v", port)
	log.Printf("start profile on port %v\n", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
		os.Exit(0)
	}
}

func main() {
	var (
		wg sync.WaitGroup
		m any = nil
	)

	//run time setup
	n := 1
	runtime.GOMAXPROCS(n)
	runtime.SetMutexProfileFraction(n)
	runtime.SetBlockProfileRate(n)

	//defer
	defer func() {
		if err := recover(); err != m {
			log.Printf("server panic, err:%v", err)
		}
		wg.Done()
		log.Printf("end server..")
	}()

	//init server
	s = tinyrpc.NewService()

	//set cb for gen and stream mode
	s.SetCBForGeneral(cbForGenReq)
	s.SetCBForStream(cbForStreamReq)

	//cb for client stat
	s.SetCBForClientNodeDown(cbForClientNodeDown)
	s.SetCBForClientNodeUp(cbForClientNodeUp)

	//start profile
	go startProfile()

	//start service
	wg.Add(1)
	log.Printf("start server on port %v..", s.GetPort())
	s.Start()
	//go sendStreamData(s)
	wg.Wait()
}
