package tinyrpc


import (
	"errors"
	"github.com/andyzhou/tinyrpc/proto"
	"log"
	"sync"
)

/*
 * Node interface for service
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 * used for manage remote rpc client address of stream mode
 */

//node face
type RpcNode struct {
	remoteStreams map[string]proto.PacketService_StreamReqServer //`remoteAddr -> stream`
	cbForNodeDown func(remoteAddr string) bool
	sync.RWMutex
}

//construct
func NewRpcNode() *RpcNode {
	this := &RpcNode{
		remoteStreams: map[string]proto.PacketService_StreamReqServer{},
	}
	return this
}

//quit
func (r *RpcNode) Quit() {
	r.Lock()
	defer r.Unlock()
	r.remoteStreams = map[string]proto.PacketService_StreamReqServer{}
}

//cast packet to streams nodes
func (r *RpcNode) CastToNodes(packet *proto.Packet, nodes ...string) error {
	var (
		stream proto.PacketService_StreamReqServer
		isOk bool
		err error
	)
	//check
	if packet == nil {
		return errors.New("invalid parameter")
	}
	if len(r.remoteStreams) <= 0 {
		return errors.New("no any nodes")
	}
	
	//cast to relate nodes
	if nodes != nil {
		for _, node := range nodes {
			stream, isOk = r.remoteStreams[node]
			if !isOk {
				continue
			}
			err = stream.Send(packet)
			if err != nil {
				log.Printf("RpcNode::CastToNodes, send to %v failed, err:%v", node, err.Error())
			}
		}
		return nil
	}

	//send to all
	for node, stream := range r.remoteStreams {
		err = stream.Send(packet)
		if err != nil {
			log.Printf("RpcNode::CastToNodes, send to %v failed, err:%v", node, err.Error())
		}
	}
	return nil
}

//clean up
func (r *RpcNode) CleanUp() {
	r.Lock()
	defer r.Unlock()
	for k, _ := range r.remoteStreams {
		delete(r.remoteStreams, k)
	}
}

//get all streams
func (r *RpcNode) GetAllStreams() map[string]proto.PacketService_StreamReqServer {
	return r.remoteStreams
}

//remove stream
func (r *RpcNode) RemoveStream(remoteAddr string) bool {
	if remoteAddr == "" {
		return false
	}
	r.Lock()
	defer r.Unlock()
	delete(r.remoteStreams, remoteAddr)
	return true
}

//get stream
func (r *RpcNode) GetStream(remoteAddr string) (proto.PacketService_StreamReqServer, error) {
	//check
	if remoteAddr == "" {
		return nil, errors.New("invalid parameter")
	}
	//get with locker
	r.Lock()
	defer r.Unlock()
	v, ok := r.remoteStreams[remoteAddr]
	if !ok || v == nil {
		return nil, errors.New("no matched stream")
	}
	return v, nil
}

//check or add remote client stream info
func (r *RpcNode) AddStream(remoteAddr string, stream proto.PacketService_StreamReqServer) error {
	//check
	if remoteAddr == "" || stream == nil {
		return errors.New("invalid parameter")
	}
	_, ok := r.remoteStreams[remoteAddr]
	if ok {
		return errors.New("node had exists")
	}
	//add new record with locker
	r.Lock()
	defer r.Unlock()
	r.remoteStreams[remoteAddr] = stream
	return nil
}

//set callback for node down
func (r *RpcNode) SetCBForNodeDown(cb func(remoteAddr string) bool) bool {
	if cb == nil {
		return false
	}
	r.cbForNodeDown = cb
	return true
}