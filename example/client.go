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

//global variables
var (
	closeChan = make(chan bool, 2)
)

//send process
func sendGenReqProcess(c *tinyrpc.Client) {
	var (
		err error
		ticker = time.NewTicker(time.Second * 5)
	)

	//check
	if c == nil {
		return
	}

	defer func() {
		ticker.Stop()
	}()

	//init request packet
	req := c.GenPacket()
	req.Module = "test"
	req.MessageId = 1
	req.Data = make([]byte, 0)

	//init response packet
	resp := c.GenPacket()

	//loop send
	for {
		select {
		case <- ticker.C:
			{
				req.Data = []byte(fmt.Sprintf("%v", time.Now().Unix()))
				resp, err = c.SendRequest(req)
				log.Printf("sendGenReqProcess, resp:%v, err:%v\n", resp, err)
			}
		case <- closeChan:
			{
				return
			}
		}
	}
}

func sendStreamReqProcess(c *tinyrpc.Client) {
	var (
		ticker = time.NewTicker(time.Second * 5)
		err error
	)
	//check
	if c == nil {
		return
	}

	defer func() {
		ticker.Stop()
		if c != nil {
			c.Quit()
			c = nil
		}
	}()

	//init request packet
	req := c.GenPacket()
	req.MessageId = 1
	req.Data = make([]byte, 0)

	//loop send
	for {
		select {
		case <-ticker.C:
			{
				req.Data = []byte(fmt.Sprintf("%v", time.Now().Unix()))
				err = c.SendStreamData(req)
				log.Printf("sendStreamReqProcess, err:%v\n", err)
			}
		case <-closeChan:
			{
				return
			}
		}
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

	//close old process
	closeChan <- true
	closeChan <- true
	time.Sleep(time.Second * 2)

	//start new client
	startNewClient()
	return nil
}

//init new client
func initNewClient() (*tinyrpc.Client, error) {
	//init client
	remoteAddr := fmt.Sprintf("%v:%v", DefaultRemoteHost, DefaultRemotePort)
	c := tinyrpc.NewClient()
	c.SetAddress(remoteAddr)
	err := c.ConnectServer()
	return c, err
}

//start new client
func startNewClient() {
	var (
		c *tinyrpc.Client
		err error
	)
	for {
		//init client
		c, err = initNewClient()
		if err != nil {
			log.Printf("connect rpc server failed, err:%v\n", err.Error())
			time.Sleep(time.Second * 2)
		}else{
			//connect success
			log.Printf("connect rpc server success..\n")
			break
		}
	}

	//set stream cb and run as stream mode
	c.SetStreamCallBack(cbForStreamData)
	c.SetServerNodeDownCallBack(cbForServiceNodeDown)

	//send gen rpc request
	go sendGenReqProcess(c)
	go sendStreamReqProcess(c)
}

func main() {
	var (
		wg sync.WaitGroup
	)
	//start send process
	wg.Add(1)
	log.Printf("start client...")

	//start new client
	startNewClient()

	wg.Wait()
	log.Printf("end client...")
}