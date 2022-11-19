package tinyrpc


import (
	"context"
	"errors"
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
 * - use stream data for communicate
 * - one node one client
 * - use outside call back for received stream data
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
	closeChan chan bool
	ctx context.Context
	sync.RWMutex
}

//construct
func NewRpcClient(address string, modes ...int) *RpcClient {
	//set mode
	mode := ModeOfRpcGen
	if modes != nil && len(modes) > 0 {
		mode = modes[0]
	}
	//self init
	this := &RpcClient{
		address:address,
		mode:mode,
		sendChan:make(chan proto.Packet, NodeDataChanSize),
		receiveChan:make(chan proto.Packet, NodeDataChanSize),
		closeChan:make(chan bool, 1),
		ctx:context.Background(),
	}

	//spawn main process
	go this.runMainProcess()
	//try ping server
	go this.ping(false)
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

	if n.mode <= ModeOfRpcGen {
		return nil
	}

	//create stream of both side
	for {
		stream, err = n.client.StreamReq(n.ctx)
		if err != nil {
			if maxTryTimes > MaxTryTimes {
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
func (n *RpcClient) receiveServerStream(stream proto.PacketService_StreamReqClient) {
	var (
		in *proto.Packet
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
		log.Printf("RpcClient::sendDataToServer, send data failed, err:%v", err.Error())
		return err
	}
	return nil
}

//check remote server status
func (n *RpcClient) checkServerStatus() {
	var (
		needPing bool
	)
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

//run main process
func (n *RpcClient) runMainProcess() {
	var (
		ticker = time.NewTicker(time.Second * NodeCheckRate)
		data proto.Packet
		needQuit, isOk bool
	)
	defer func() {
		if err := recover(); err != nil {
			log.Printf("RpcClient::runMainProcess panic, err:%v", err)
		}
		ticker.Stop()
		log.Printf("RpcClient::runMainProcess of %v need quit..", n.address)
	}()

	//loop
	for {
		if needQuit && len(n.sendChan) <= 0 {
			break
		}
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
		case <- ticker.C:
			//check server status
			n.checkServerStatus()
		case <- n.closeChan:
			return
		}
	}
}