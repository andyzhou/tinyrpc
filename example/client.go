package main

import (
	"fmt"
	"github.com/andyzhou/tinyrpc"
	"github.com/andyzhou/tinyrpc/define"
	"github.com/andyzhou/tinyrpc/proto"
	"log"
	"os"
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
func sendGenReqProcess(c *tinyrpc.Client) {
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

func sendStreamReqProcess(c *tinyrpc.Client) {
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
func cbForStreamData(pack *proto.Packet) error {
	log.Printf("cbForStreamData, pack data:%v", pack.Data)
	return nil
}

//set cb for service node down
func cbForServiceNodeDown(node string) error {
	log.Printf("cbForServiceNodeDown, node:%v\n", node)
	return nil
}

func main() {
	var (
		wg sync.WaitGroup
	)

	//init client
	remoteAddr := fmt.Sprintf("%v:%v", DefaultRemoteHost, DefaultRemotePort)
	c := tinyrpc.NewClient(define.ModeOfRpcAll)
	c.SetAddress(remoteAddr)
	err := c.ConnectServer()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	//send gen rpc request
	go sendGenReqProcess(c)

	//set stream cb and run as stream mode
	c.SetStreamCallBack(cbForStreamData)
	c.SetServerNodeDownCallBack(cbForServiceNodeDown)
	go sendStreamReqProcess(c)

	//start send process
	wg.Add(1)
	log.Printf("start client...")
	wg.Wait()
	log.Printf("end client...")
}