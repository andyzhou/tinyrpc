package testing

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tinyrpc"
	"github.com/andyzhou/tinyrpc/proto"
	"math/rand"
	"sync"
	"testing"
	"time"
)

//inter macro define
const (
	DefaultRemoteHost = "127.0.0.1"
	DefaultRemotePort = 7100
	MaxClients = 9
)

//global variables
var (
	clients map[int]*tinyrpc.Client
	err error
	locker sync.RWMutex
)

//init
func init() {
	clients = map[int]*tinyrpc.Client{}
	for i := 0; i < MaxClients; i++ {
		client, subErr := initClient()
		if subErr != nil {
			panic(any(subErr))
			return
		}
		clients[i] = client
	}
}

//init new client
func initClient() (*tinyrpc.Client, error) {
	//init client
	remoteAddr := fmt.Sprintf("%v:%v", DefaultRemoteHost, DefaultRemotePort)
	c := tinyrpc.NewClient()
	c.SetAddress(remoteAddr)
	subErr := c.ConnectServer()
	return c, subErr
}

//get rand client
func getRandClient() *tinyrpc.Client {
	//get rand idx
	now := time.Now().UnixNano()
	rand.Seed(now)
	randIdx := rand.Intn(MaxClients)

	//get target client
	//locker.Lock()
	//defer locker.Unlock()
	client, ok := clients[randIdx]
	if ok && client != nil {
		return client
	}
	return nil
}

//send gen rpc request
func sendGenReq() (*proto.Packet, error){
	//check
	client := getRandClient()
	if client == nil {
		return nil, errors.New("can't get client")
	}

	//init request packet
	req := client.GenPacket()
	req.Module = "test"
	req.MessageId = 1
	req.Data = make([]byte, 0)
	req.Data = []byte(fmt.Sprintf("%v", time.Now().Unix()))
	req.IsCast = true

	//send request
	resp, subErr := client.SendRequest(req)
	return resp, subErr
}

//test rpc
func TestRpc(t *testing.T) {
	resp, subErr := sendGenReq()
	if subErr != nil {
		t.Errorf("test rpc failed, err:%v\n", subErr.Error())
		return
	}
	t.Logf("test rpc succeed, resp:%v\n", resp)
}

//benchmark rpc
func BenchmarkRpc(b *testing.B) {
	succeed := 0
	failed := 0
	for i := 0; i < b.N; i++ {
		_, subErr := sendGenReq()
		if subErr != nil {
			failed++
		}else{
			succeed++
		}
	}
	b.Logf("benchmark rpc result, succeed:%v, failed:%v\n", succeed, failed)
}