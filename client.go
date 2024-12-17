package tinyrpc

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/andyzhou/tinyrpc/define"
	"github.com/andyzhou/tinyrpc/proto"
	"github.com/andyzhou/tinyrpc/util"
	"google.golang.org/grpc"
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
	Mode       int //ModeOfRpcGen, ModeOfRpcStream, ModeOfRpcAll
	MaxMsgSize int //default 4MB
	TimeOut    int //xx seconds
	AutoReConn bool
}

//rpc client face
type Client struct {
	para             *ClientPara                         //rpc client para
	address          string                              //node remote address
	conn             *grpc.ClientConn                    //rpc connect
	client           proto.PacketServiceClient           //service client
	stream           proto.PacketService_StreamReqClient //stream packet client
	receiveChan      chan proto.Packet                   //packet chan for receive outside
	sendChan         chan proto.Packet                   //packet chan for send
	receiveCloseChan chan struct{}
	closeChan        chan struct{}
	hasRun           bool
	streamTimeOut    int //xx seconds, 0 means no limit.
	connCtx context.Context
	connCancel context.CancelFunc
	//callback
	cbOfStream   func(packet *proto.Packet) error //cb for received stream data of outside
	cbOfNodeDown func(node string) error          //cb for service node down
	util.Util
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
			TimeOut: define.DefaultConnTimeOut,
			AutoReConn: true,
		}
	}
	if para.TimeOut <= 0 {
		para.TimeOut = define.DefaultConnTimeOut
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
	}
	return this
}

///////
//api
///////

//quit
func (n *Client) Quit() {
	if n.closeChan != nil {
		n.closeChan <- struct{}{}
		time.Sleep(time.Second)
		n.closeChan <- struct{}{}
		time.Sleep(time.Second)
	}
	if n.conn != nil {
		n.conn.Close()
	}
	if n.connCancel != nil {
		n.connCancel()
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
		if n.para.AutoReConn {
			time.Sleep(time.Second)
			return n.ConnectServer()
		}
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
func (n *Client) SendRequest(
		req *proto.Packet,
		needChecks ...bool,
	) (*proto.Packet, error) {
	var (
		needCheck bool
	)
	//check
	if req == nil || n.client == nil {
		return nil, errors.New("invalid parameter")
	}
	if needChecks != nil && len(needChecks) > 0 {
		needCheck = needChecks[0]
	}

	//check connect lost or not
	if needCheck {
		bRet := n.checkServerConn()
		if !bRet {
			return nil, errors.New("client lost connect")
		}
	}

	//create context
	ctx, cancel := n.createContext()
	defer cancel()

	//send request
	resp, err := n.client.SendReq(ctx, req)
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

	chanClosed, _ := n.IsChanClosed(n.sendChan)
	if chanClosed {
		log.Printf("RpcClient::SendData, chan has closed\n")
	}else{
		//send to data chan
		select {
		case n.sendChan <- *data:
		}
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

//set stream receive timeout
//0 means no limit
func (n *Client) SetStreamReceiveTimeOut(seconds int) bool {
	if seconds < 0 {
		return false
	}
	n.streamTimeOut = seconds
	return true
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

//create context
func (n *Client) createContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(
		context.Background(),
		time.Duration(n.para.TimeOut) * time.Second)
}

//ping remote server
func (n *Client) ping(isReConnects... bool) error {
	var (
		stream proto.PacketService_StreamReqClient
		err error
		isFailed bool
		maxTryTimes int
		isReConn bool
		ctx context.Context
		cancel context.CancelFunc
	)
	//check
	if isReConnects != nil && len(isReConnects) > 0 {
		isReConn = isReConnects[0]
	}
	if isReConn {
		if n.conn != nil {
			if n.connCancel != nil {
				n.connCancel()
			}
			n.conn.Close()
			n.conn = nil
		}
	}
	if n.address == "" {
		return errors.New("didn't setup server address")
	}

	//check and init rpc connect
	if n.conn == nil {
		n.connCtx, n.connCancel = context.WithTimeout(
						context.Background(),
						time.Duration(n.para.TimeOut) * time.Second)

		//connect remote server
		conn, subErr := grpc.DialContext(
					n.connCtx,
					n.address,
					grpc.WithBlock(), //wait block until shake hand succeed
					grpc.WithInsecure(),
					grpc.WithDefaultCallOptions(
						grpc.MaxCallSendMsgSize(n.para.MaxMsgSize),
						grpc.MaxCallRecvMsgSize(n.para.MaxMsgSize),
					),
				)
		if subErr != nil {
			log.Printf("Can't pind %v, err:%v",  n.address, subErr.Error())
			if n.connCancel != nil {
				n.connCancel()
			}
			n.hasRun = false
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

	//setup timeout for receive stream
	if n.streamTimeOut > 0 {
		ctx, cancel = context.WithTimeout(
			context.Background(),
			time.Second * time.Duration(n.streamTimeOut))
	}else{
		//forever
		ctx = context.Background()
	}

	//create stream of both side
	for {
		//stream, err = n.client.StreamReq(n.ctx)
		stream, err = n.client.StreamReq(ctx)
		if err != nil {
			if maxTryTimes > define.MaxTryTimes {
				//cancel context
				if cancel != nil {
					cancel()
				}
				return err
				break
			}
			log.Printf("Create stream with server %v failed, err:%v", n.address, err.Error())
			time.Sleep(time.Second)
			maxTryTimes++
			continue
		}
		break
	}

	if isFailed {
		//cancel context
		if cancel != nil {
			cancel()
		}
		return err
	}

	//sync stream object
	n.stream = stream

	//spawn stream receiver
	go n.receiveServerStream(stream, ctx, cancel)
	return nil
}

//receive stream data from server
func (n *Client) receiveServerStream(
		stream proto.PacketService_StreamReqClient,
		ctx context.Context,
		cancel context.CancelFunc,
	) {
	var (
		in *proto.Packet
		err error
		m any = nil
	)

	//try catch panic
	defer func() {
		if subErr := recover(); subErr != m {
			log.Printf("RpcClient::receiveServerStream, panic happened, err:%v", subErr)
		}
		//context cancel
		if cancel != nil {
			cancel()
		}
		log.Printf("RpcClient::receiveServerStream process ended!\n")
	}()

	//receive data use for loop
	for {
		//receive data
		in, err = stream.Recv()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("RpcClient::receiveServerStream, Receive data time out.\n")
				break
			}
			if err == io.EOF {
				continue
			}
			log.Printf("RpcClient::receiveServerStream, Receive data failed, err:%v", err.Error())
			break
		}
		//send data to receive chan of outside
		closed, _ := n.IsChanClosed(n.receiveChan)
		if closed {
			log.Printf("RpcClient::receiveServerStream, receive data chan is closed\n")
			break
		}else{
			if n.receiveChan != nil {
				select {
				case n.receiveChan <- *in:
				}
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
			//server node down
			if n.cbOfNodeDown != nil {
				n.cbOfNodeDown(n.address)
			}
			needPing = true
			n.hasRun = false
		}
	}
	if needPing {
		n.ping(true)
	}
}

//check remote connect is lost or not
func (n *Client) checkServerConn() bool {
	if n.conn == nil {
		return false
	}
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
		log.Printf("RpcClient::tickerProcess ended!\n")
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
		//close(n.sendChan)
		close(n.receiveChan)
		log.Printf("RpcClient::runMainProcess ended!\n")
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