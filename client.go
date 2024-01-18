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

//rpc client para
type ClientPara struct {
	Mode int //ModeOfRpcGen, ModeOfRpcStream, ModeOfRpcAll
	MaxMsgSize int //default 4MB
}

//rpc client face
type Client struct {
	para *ClientPara //rpc client para
	address string //node remote address
	conn *grpc.ClientConn //rpc connect
	client proto.PacketServiceClient //service client
	stream proto.PacketService_StreamReqClient //stream packet client
	receiveChan chan proto.Packet //packet chan for receive outside
	sendChan chan proto.Packet //packet chan for send
	receiveCloseChan chan struct{}
	closeChan chan struct{}
	hasRun bool
	ctx context.Context
	//callback
	cbOfStream func(packet *proto.Packet)error //cb for received stream data of outside
	cbOfNodeDown func(node string) error //cb for service node down
	sync.RWMutex
}

//construct
func NewClient(paras ...*ClientPara) *Client {
	var (
		para *ClientPara
	)

	//detect para
	if paras != nil && len(paras) > 0 {
		para = paras[0]
	}else{
		para = &ClientPara{
			Mode: define.ModeOfRpcAll,
			MaxMsgSize: 1024 * 1024 * 4, //4MB
		}
	}

	//set default mode
	if para.Mode > define.ModeOfRpcAll || para.Mode < define.ModeOfRpcGen {
		para.Mode = define.ModeOfRpcAll
	}

	//self init
	this := &Client{
		para: para,
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
func (n *Client) Quit() {
	if n.closeChan != nil {
		close(n.closeChan)
	}
}

//set message max size
func (n *Client) SetMsgMaxSize(size int) error {
	if size <= 0 {
		return errors.New("invalid size")
	}
	n.para.MaxMsgSize = size
	return nil
}

//set mode
func (n *Client) SetMode(mode int) error {
	if mode > define.ModeOfRpcAll || mode < define.ModeOfRpcGen {
		return errors.New("invalid mode")
	}
	n.para.Mode= mode
	return nil
}

//set address
func (n *Client) SetAddress(address string) error{
	if address == "" {
		return errors.New("invalid parameter")
	}
	n.address = address
	return nil
}

//connect server
func (n *Client) ConnectServer() error {
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
func (n *Client) SendRequest(req *proto.Packet) (*proto.Packet, error) {
	//check
	if req == nil || n.client == nil {
		return nil, errors.New("invalid parameter")
	}

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
func (n *Client) SendStreamData(data *proto.Packet) error {
	var (
		m any = nil
	)
	//check
	if data == nil {
		return errors.New("invalid parameter")
	}
	if n.sendChan == nil {
		return errors.New("inter send chan is nil")
	}

	//try catch panic
	defer func() {
		if err := recover(); err != m {
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
func (n *Client) GenPacket() *proto.Packet {
	return &proto.Packet{}
}

//set callback for received stream data of outside
func (n *Client) SetStreamCallBack(cb func(*proto.Packet)error) {
	n.cbOfStream = cb
}

//set callback for server node down
func (n *Client) SetServerNodeDownCallBack(cb func(string) error) {
	n.cbOfNodeDown = cb
}

//check server connect
func (n *Client) CheckConn() bool {
	return n.checkServerConn()
}

//ping remote server
func (n *Client) Ping(isReConn ...bool) error {
	return n.ping(isReConn...)
}

/////////////////
//private func
////////////////

//ping remote server
func (n *Client) ping(isReConnects... bool) error {
	var (
		stream proto.PacketService_StreamReqClient
		err error
		isFailed bool
		maxTryTimes int
		isReConn bool
	)
	//check
	if isReConnects != nil && len(isReConnects) > 0 {
		isReConn = isReConnects[0]
	}
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
		//connect remote server
		conn, subErr := grpc.Dial(
					n.address,
					grpc.WithInsecure(),
					grpc.WithDefaultCallOptions(
						grpc.MaxCallSendMsgSize(n.para.MaxMsgSize),
						grpc.MaxCallRecvMsgSize(n.para.MaxMsgSize),
					),
				)
		if subErr != nil {
			log.Printf("Can't pind %v, err:%v",  n.address, err.Error())
			return subErr
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
	if n.para.Mode <= define.ModeOfRpcGen {
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
		//log.Printf("Create stream with server %v success", n.address)
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
func (n *Client) receiveServerStream(
				stream proto.PacketService_StreamReqClient,
			) {
	var (
		in *proto.Packet
		err error
		m any = nil
	)

	//try catch panic
	defer func() {
		if subErr := recover(); subErr != m {
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
func (n *Client) sendDataToServer(data *proto.Packet) error {
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
func (n *Client) checkServerStatus() {
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
			//node down
			if n.cbOfNodeDown != nil {
				n.cbOfNodeDown(n.address)
			}
			needPing = true
		}
	}
	if needPing {
		n.ping(true)
	}
}

//check remote connect is lost or not
func (n *Client) checkServerConn() bool {
	state := n.conn.GetState().String()
	if state == "TRANSIENT_FAILURE" || state == "SHUTDOWN" {
		return false
	}
	return true
}

//ticker process
func (n *Client) tickerProcess() {
	var (
		ticker = time.NewTicker(define.NodeCheckRate)
		m any = nil
	)
	defer func() {
		if err := recover(); err != m {
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
func (n *Client) runMainProcess() {
	var (
		data proto.Packet
		isOk bool
		m any = nil
	)

	//defer
	defer func() {
		if err := recover(); err != m {
			log.Printf("client::runMainProcess panic, err:%v", err)
		}
		//close relate chan
		close(n.sendChan)
		close(n.receiveChan)
	}()

	//loop
	for {
		select {
		case data, isOk = <- n.sendChan:
			if isOk {
				//send to remote server
				n.sendDataToServer(&data)
			}
		case data, isOk = <- n.receiveChan:
			if isOk && &data != nil {
				//run callback func to process received data
				if n.cbOfStream != nil {
					n.cbOfStream(&data)
				}
			}
		case <- n.closeChan:
			return
		}
	}
}