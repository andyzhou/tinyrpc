package tinyrpc

import (
	"context"
	"errors"
	"github.com/andyzhou/tinyrpc/define"
	"github.com/andyzhou/tinyrpc/proto"
	"google.golang.org/grpc"
	"io"
	"log"
	"sync"
	"time"
)

/*
 * rpc client interface
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 *
 * - support gen and stream data for communicate
 * - use outside call back for received stream data
 * - one node, one client
 */

//rpc client face
type RpcClient struct {
	address string `node remote address`
	mode int `rpc mode`
	cb func(packet *proto.Packet)bool `call back for received stream data of outside`
	conn *grpc.ClientConn `rpc connect`
	client proto.PacketServiceClient `service client`
	stream proto.PacketService_StreamReqClient `stream packet client`
	receiveChan chan proto.Packet `packet chan for receive outside`
	sendChan chan proto.Packet `packet chan for send`
	receiveCloseChan chan struct{}
	closeChan chan struct{}
	hasRun bool
	ctx context.Context
	sync.RWMutex
}

//construct
func NewRpcClient(modes ...int) *RpcClient {
	//set default mode
	mode := define.ModeOfRpcAll
	if modes != nil && len(modes) > 0 {
		mode = modes[0]
	}

	//self init
	this := &RpcClient{
		mode: mode,
		sendChan:make(chan proto.Packet, define.NodeDataChanSize),
		receiveChan:make(chan proto.Packet, define.NodeDataChanSize),
		receiveCloseChan:make(chan struct{}, 1),
		closeChan:make(chan struct{}, 2),
		ctx:context.Background(),
	}
	return this
}

///////
//api
///////

//quit
func (n *RpcClient) Quit() {
	if n.closeChan != nil {
		close(n.closeChan)
	}
}

//set address
func (n *RpcClient) SetAddress(address string) error{
	if address == "" {
		return errors.New("invalid parameter")
	}
	n.address = address
	return nil
}

//try connect server
func (n *RpcClient) ConnectServer() error {
	//check
	if n.address == "" {
		return errors.New("server address didn't setup")
	}
	err := n.ping(true)
	if err != nil {
		return err
	}
	if !n.hasRun {
		//spawn relate process
		go n.tickerProcess()
		go n.runMainProcess()
		n.hasRun = true
	}
	return nil
}

//send general data to remote server
//sync mode
func (n *RpcClient) SendRequest(req *proto.Packet) (*proto.Packet, error) {
	//check
	if req == nil || n.client == nil {
		return nil, errors.New("invalid parameter")
	}

	//try catch panic
	defer func() {
		if err := recover(); err != nil {
			log.Printf("RpcClient::SendRequest panic happened, err:%v", err)
		}
	}()

	//check connect lost or not
	bRet := n.checkServerConn()
	if !bRet {
		return nil, errors.New("client lost connect")
	}

	//send request
	resp, err := n.client.SendReq(context.Background(), req)
	return resp, err
}

//send stream data to remote server
//async mode
func (n *RpcClient) SendStreamData(data *proto.Packet) error {
	//check
	if data == nil {
		return errors.New("invalid parameter")
	}

	//try catch panic
	defer func() {
		if err := recover(); err != nil {
			log.Printf("RpcClient::SendData, panic happened, err:%v", err)
		}
	}()

	//send to data chan
	select {
	case n.sendChan <- *data:
	}
	return nil
}

//gen new packet
func (n *RpcClient) GenPacket() *proto.Packet {
	return &proto.Packet{}
}

//set call back for received stream data of outside
func (n *RpcClient) SetStreamCallBack(cb func(*proto.Packet)bool) {
	n.cb = cb
}

//check server connect
func (n *RpcClient) CheckConn() bool {
	return n.checkServerConn()
}

/////////////////
//private func
////////////////

//ping remote server
func (n *RpcClient) ping(isReConn bool) error {
	var (
		stream proto.PacketService_StreamReqClient
		err error
		isFailed bool
		maxTryTimes int
	)
	if isReConn {
		if n.conn != nil {
			n.conn.Close()
			n.conn = nil
		}
	}
	if n.address == "" {
		return errors.New("didn't setup server address")
	}

	//check and init rpc connect
	if n.conn == nil {
		//try connect remote server
		conn, err := grpc.Dial(n.address, grpc.WithInsecure())
		if err != nil {
			log.Printf("Can't pind %v, err:%v",  n.address, err.Error())
			return err
		}

		//set rpc client and connect
		n.Lock()
		n.client = proto.NewPacketServiceClient(conn)
		n.conn = conn
		n.Unlock()
	}

	//force close last stream receiver
	if n.receiveCloseChan != nil {
		n.receiveCloseChan <- struct{}{}
	}

	//if only gen rpc, do nothing
	if n.mode <= define.ModeOfRpcGen {
		return nil
	}

	//create stream of both side
	for {
		stream, err = n.client.StreamReq(n.ctx)
		if err != nil {
			if maxTryTimes > define.MaxTryTimes {
				return err
				break
			}
			log.Printf("Create stream with server %v failed, err:%v", n.address, err.Error())
			time.Sleep(time.Second)
			maxTryTimes++
			continue
		}
		log.Printf("Create stream with server %v success", n.address)
		break
	}

	if isFailed {
		return err
	}

	//sync stream object
	n.stream = stream

	//spawn stream receiver
	go n.receiveServerStream(stream)
	return nil
}

//receive stream data from server
func (n *RpcClient) receiveServerStream(
				stream proto.PacketService_StreamReqClient,
			) {
	var (
		in *proto.Packet
		isOk bool
		err error
	)

	//try catch panic
	defer func() {
		if err := recover(); err != nil {
			log.Printf("RpcClient::receiveServerStream, panic happened, err:%v", err)
		}
	}()

	//receive data use for loop
	for {
		//try get close chan
		_, isOk = <- n.closeChan
		if isOk {
			break
		}
		//receive data
		in, err = stream.Recv()
		if err != nil {
			if err == io.EOF {
				continue
			}
			log.Printf("RpcClient::receiveServerStream, Receive data failed, err:%v", err.Error())
			break
		}
		//send data to receive chan of outside
		if n.receiveChan != nil {
			select {
			case n.receiveChan <- *in:
			}
		}
	}
}

//send data to remote server
func (n *RpcClient) sendDataToServer(data *proto.Packet) error {
	//check
	if data == nil || n.stream == nil {
		return errors.New("invalid parameter")
	}
	//send stream data
	err := n.stream.Send(data)
	if err != nil {
		log.Printf("RpcClient::sendDataToServer, send data failed, err:%v\n", err.Error())
		return err
	}
	return nil
}

//check remote server status
func (n *RpcClient) checkServerStatus() {
	var (
		needPing bool
	)
	if n.address == "" {
		return
	}
	if n.conn == nil {
		//try reconnect
		needPing = true
	}else{
		state := n.conn.GetState().String()
		if state == "TRANSIENT_FAILURE" || state == "SHUTDOWN" {
			needPing = true
		}
	}
	if needPing {
		n.ping(true)
	}
}

//check remote connect is lost or not
func (n *RpcClient) checkServerConn() bool {
	state := n.conn.GetState().String()
	if state == "TRANSIENT_FAILURE" || state == "SHUTDOWN" {
		return false
	}
	return true
}

//ticker process
func (n *RpcClient) tickerProcess() {
	var (
		ticker = time.NewTicker(time.Second * define.NodeCheckRate)
	)
	defer func() {
		if err := recover(); err != nil {
			log.Printf("RpcClient::tickerProcess panic, err:%v", err)
		}
		//stop ticker
		ticker.Stop()
	}()

	//check server first time
	n.checkServerStatus()

	//loop
	for {
		select {
		case <- ticker.C:
			//check server status
			n.checkServerStatus()
		case <- n.closeChan:
			return
		}
	}
}

//run main process
//used for send and receiver stream data
func (n *RpcClient) runMainProcess() {
	var (
		data proto.Packet
		isOk bool
	)
	defer func() {
		if err := recover(); err != nil {
			log.Printf("RpcClient::runMainProcess panic, err:%v", err)
		}
		//close relate chan
		close(n.sendChan)
		close(n.receiveChan)
		close(n.closeChan)
		log.Printf("RpcClient::runMainProcess of %v need quit..", n.address)
	}()

	//loop
	for {
		select {
		case data, isOk = <- n.sendChan:
			if isOk {
				//try send to remote server
				n.sendDataToServer(&data)
			}
		case data, isOk = <- n.receiveChan:
			if isOk {
				//run callback func to process received data
				if n.cb != nil {
					n.cb(&data)
				}
			}
		case <- n.closeChan:
			return
		}
	}
}