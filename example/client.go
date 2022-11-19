package main

import (
	"fmt"
	"github.com/andyzhou/tinyrpc"
	"github.com/andyzhou/tinyrpc/proto"
	"log"
	"sync"
	"time"
)

//example code for client

//inter macro define
const (
	DefaultRemoteHost = "127.0.0.1"
	DefaultRemotePort = 7100
)

//send process
func sendGenReqProcess(c *tinyrpc.RpcClient) {
	var (
		err error
	)

	//check
	if c == nil {
		return
	}

	//init request packet
	req := c.GenPacket()
	req.MessageId = 1
	req.Data = make([]byte, 0)

	//init response packet
	resp := c.GenPacket()

	//loop send
	for {
		req.Data = []byte(fmt.Sprintf("%v", time.Now().Unix()))
		resp, err = c.SendRequest(req)
		log.Printf("sendGenReqProcess, resp:%v, err:%v\n", resp, err)
		time.Sleep(time.Second * 2)
	}
}

func sendStreamReqProcess(c *tinyrpc.RpcClient) {
	var (
		err error
	)
	//check
	if c == nil {
		return
	}

	//init request packet
	req := c.GenPacket()
	req.MessageId = 1
	req.Data = make([]byte, 0)

	//loop send
	for {
		req.Data = []byte(fmt.Sprintf("%v", time.Now().Unix()))
		err = c.SendStreamData(req)
		log.Printf("sendStreamReqProcess, err:%v\n", err)
		time.Sleep(time.Second * 2)
	}
}

//set cb for stream data
func cbForStreamData(pack *proto.Packet) bool {
	log.Printf("cbForStreamData, pack data:%v", pack.Data)
	return true
}

func main() {
	var (
		wg sync.WaitGroup
	)

	//init client
	remoteAddr := fmt.Sprintf("%v:%v", DefaultRemoteHost, DefaultRemotePort)
	c := tinyrpc.NewRpcClient(remoteAddr, tinyrpc.ModeOfRpcAll)
	c.SetStreamCallBack(cbForStreamData)

	//start send process
	wg.Add(1)
	log.Printf("start client...")
	//go sendGenReqProcess(c)
	go sendStreamReqProcess(c)
	wg.Wait()
	log.Printf("end client...")
}