package rpc

import (
	"fmt"
	"github.com/andyzhou/tinyrpc/define"
	"github.com/andyzhou/tinyrpc/proto"
	"golang.org/x/net/context"
	"io"
	"log"
	"sync"
)

/*
 * RPC service callback for service
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 *
 * - support general and stream mode
 */

//callback face
type CallBack struct {
	//cb for stream data
	streamCB func(string,*proto.Packet) error
	//cb for general data
	generalCB func(string,*proto.Packet)(*proto.Packet, error)
	nodeFace *Node //nodes
	sync.RWMutex
}

//construct
func NewCallBack(nodeFace *Node) *CallBack {
	this := &CallBack{
		nodeFace:nodeFace,
	}
	return this
}

//set callback for stream request
func (r *CallBack) SetCBForStream(cb func(string,*proto.Packet)error) {
	r.streamCB = cb
}

//set callback for general request
func (r *CallBack) SetCBForGen(cb func(string,*proto.Packet)(*proto.Packet, error)) {
	r.generalCB = cb
}

//receive stream data from client
func (r *CallBack) StreamReq(
				stream proto.PacketService_StreamReqServer,
			) error {
	var (
		in *proto.Packet
		err error
		m any = nil
	)

	//get context
	ctx := stream.Context()

	//get tag by stream
	tag, ok := RunStat.GetConnTagFromContext(ctx)
	if !ok {
		err = fmt.Errorf("StreamReq, can't get tag from node stream")
		log.Println(err)
		return err
	}

	//check or sync remote rpc client info
	remoteAddr := tag.RemoteAddr.String()
	if r.nodeFace != nil {
		log.Printf("service.RpcCallBack:StreamReq, add stream, remoteAddr:%v\n", remoteAddr)
		r.nodeFace.AddStream(remoteAddr, stream)
	}

	//defer
	defer func() {
		if subErr := recover(); subErr != m {
			log.Printf("service.RpcCallBack:StreamReq panic, err:%v\n", err)
		}
	}()

	//node up
	r.nodeFace.AddStream(remoteAddr, stream)

	//try receive stream data from node
	for {
		select {
		case <- ctx.Done():
			{
				log.Println("service.RpcCallBack:StreamReq, Receive down signal from client")
				return ctx.Err()
			}
		default:
			{
				in, err = stream.Recv()
				if err != nil {
					log.Printf("service.RpcCallBack:StreamReq, in:%v, err:%v\n", err, in)
					if err == io.EOF {
						return nil
					}
					return err
				}
				//received real packet data from client, process it.
				//run callback of outside rpc server
				if r.streamCB != nil {
					r.streamCB(remoteAddr, in)
				}
			}
		}
	}
	return nil
}

//receive general request from client
func (r *CallBack) SendReq(
				ctx context.Context,
				in *proto.Packet,
			) (*proto.Packet, error) {
	var (
		remoteAddr string
		errMsg string
	)
	//check
	if in == nil {
		errMsg = "lost parameter data"
		in.ErrCode = define.ErrCodeOfInvalidPara
		in.ErrMsg = errMsg
		return in, fmt.Errorf(errMsg)
	}
	if r.generalCB == nil {
		errMsg = "didn't setup general callback"
		in.ErrCode = define.ErrCodeOfNoCallBack
		in.ErrMsg = errMsg
		return in, fmt.Errorf(errMsg)
	}

	//run callback of outside to process general data
	//get tag by ctx
	tag, ok := RunStat.GetConnTagFromContext(ctx)
	if ok {
		remoteAddr = tag.RemoteAddr.String()
	}

	//call general callback
	out, err := r.generalCB(remoteAddr, in)
	if err != nil {
		out.ErrCode = define.ErrCodeOfRunError
		out.ErrMsg = err.Error()
		return in, err
	}
	return out, nil
}