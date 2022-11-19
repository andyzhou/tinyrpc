package tinyrpc


import (
	"errors"
	"fmt"
	"github.com/andyzhou/tinyrpc/proto"
	"golang.org/x/net/context"
	"io"
	"log"
	"sync"
	"time"
)

/*
 * RPC service callback for service
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 */

//callback face
type RpcCallBack struct {
	//callback for service from outside
	streamCB func(string,[]byte)bool `cb for stream data`
	//callback for service from outside
	generalCB func(string,[]byte)[]byte `cb for general data` //input/return packet data
	nodeFace *RpcNode `node interface from outside`
	sync.RWMutex
}

//construct
func NewRpcCallBack(nodeFace *RpcNode) *RpcCallBack {
	this := &RpcCallBack{
		nodeFace:nodeFace,
	}
	return this
}

//set callback for stream request
func (r *RpcCallBack) SetCBForStream(cb func(string,[]byte)bool) {
	r.streamCB = cb
}

//set callback for general request
func (r *RpcCallBack) SetCBForGen(cb func(string,[]byte)[]byte) {
	r.generalCB = cb
}

//grpc call back for stream data from client
func (r *RpcCallBack) StreamReq(stream proto.PacketService_StreamReqServer) error {
	var (
		in *proto.Packet
		err error
		tips string
	)

	//get tag by stream
	tag, ok := RunRpcStat.GetConnTagFromContext(stream.Context())
	if !ok {
		tips = "Can't get tag from node stream."
		log.Println(tips)
		return errors.New(tips)
	}

	//check or sync remote rpc client info
	if r.nodeFace != nil {
		r.nodeFace.AddStream(tag.RemoteAddr.String(), stream)
	}

	//try receive stream data from node
	for {
		in, err = stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		//received real packet data from client, process it.
		//run callback of outside rpc server
		if r.streamCB != nil {
			r.streamCB(tag.RemoteAddr.String(), in.Data)
		}
		time.Sleep(time.Second/10)
	}
	return nil
}

//grpc call back for general request
func (r *RpcCallBack) SendReq(ctx context.Context, in *proto.Packet) (*proto.Packet, error) {
	var (
		remoteAddr string
	)
	//check
	if in == nil {
		errMsg := "lost parameter data"
		in.ErrMsg = errMsg
		return in, fmt.Errorf(errMsg)
	}

	//run callback of outside to process general data
	if r.generalCB != nil {
		//get tag by stream
		tag, ok := RunRpcStat.GetConnTagFromContext(ctx)
		if ok {
			remoteAddr = tag.RemoteAddr.String()
		}
		log.Println("GRPCCallBack::SendReq, remoteAddr:", remoteAddr, ", ok:", ok)
		packetData := r.generalCB(remoteAddr, in.Data)
		in.Data = packetData
	}

	return in, nil
}