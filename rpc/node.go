package rpc

import (
	"errors"
	"log"
	"runtime"
	"sync"

	"github.com/andyzhou/tinyrpc/proto"
)

/*
 * Node interface for service
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 *
 * used for manage remote rpc client address of stream mode
 */

//node face
type Node struct {
	remoteStreams map[string]proto.PacketService_StreamReqServer //`remoteAddr -> stream`
	cbForNodeDown func(remoteAddr string) bool
	sync.RWMutex
}

//construct
func NewNode() *Node {
	this := &Node{
		remoteStreams: map[string]proto.PacketService_StreamReqServer{},
	}
	return this
}

//quit
func (r *Node) Quit() {
	r.Lock()
	defer r.Unlock()
	r.remoteStreams = map[string]proto.PacketService_StreamReqServer{}

	//gc memory
	runtime.GC()
}

//cast packet to streams nodes
func (r *Node) CastToNodes(
		packet *proto.Packet,
		nodes ...string,
	) error {
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
		return nil
	}
	
	//cast to relate nodes with locker
	if nodes != nil {
		r.Lock()
		defer r.Unlock()
		for _, node := range nodes {
			stream, isOk = r.remoteStreams[node]
			if !isOk || stream == nil {
				continue
			}
			err = stream.Send(packet)
			if err != nil {
				log.Printf("RpcNode::CastToNodes, send to %v failed, err:%v\n",
					node, err.Error())
			}
		}
		return nil
	}

	//send to all with lock
	r.Lock()
	defer r.Unlock()
	for node, subStream := range r.remoteStreams {
		err = subStream.Send(packet)
		if err != nil {
			log.Printf("RpcNode::CastToNodes, send to %v failed, err:%v\n",
				node, err.Error())
		}
	}
	return nil
}

//clean up
func (r *Node) CleanUp() {
	r.Lock()
	defer r.Unlock()
	for k, _ := range r.remoteStreams {
		delete(r.remoteStreams, k)
	}
	r.remoteStreams = map[string]proto.PacketService_StreamReqServer{}
	runtime.GC()
}

//get all streams
func (r *Node) GetAllStreams() map[string]proto.PacketService_StreamReqServer {
	r.Lock()
	defer r.Unlock()
	return r.remoteStreams
}

//remove stream
func (r *Node) RemoveStream(remoteAddr string) bool {
	//check
	if remoteAddr == "" {
		return false
	}
	//remove from map
	r.Lock()
	defer r.Unlock()
	delete(r.remoteStreams, remoteAddr)
	if len(r.remoteStreams) <= 0 {
		runtime.GC()
	}
	return true
}

//get stream
func (r *Node) GetStream(
		remoteAddr string,
	) (proto.PacketService_StreamReqServer, error) {
	//check
	if remoteAddr == "" {
		return nil, errors.New("invalid parameter")
	}
	if r.remoteStreams == nil || len(r.remoteStreams) <= 0 {
		return nil, errors.New("no any remote streams")
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
func (r *Node) AddStream(
		remoteAddr string,
		stream proto.PacketService_StreamReqServer,
	) error {
	//check
	if remoteAddr == "" || stream == nil {
		return errors.New("invalid parameter")
	}

	//check and add record with locker
	r.Lock()
	defer r.Unlock()
	_, ok := r.remoteStreams[remoteAddr]
	if ok {
		return errors.New("node had exists")
	}

	//add new record
	r.remoteStreams[remoteAddr] = stream
	return nil
}

//set callback for node down
func (r *Node) SetCBForNodeDown(
		cb func(remoteAddr string) bool,
	) bool {
	if cb == nil {
		return false
	}
	r.cbForNodeDown = cb
	return true
}