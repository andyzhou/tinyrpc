package tinyrpc

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tinyrpc/define"
	"github.com/andyzhou/tinyrpc/proto"
	"github.com/andyzhou/tinyrpc/rpc"
	"google.golang.org/grpc"
	"net"
	"sync"
)

/*
 * rpc service interface
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 *
 * support general and stream mode
 */

//rpc service face
type RpcService struct {
	port int `rpc port`
	address string `rpc service address`
	listener net.Listener `tcp listener`
	service *grpc.Server
	rpcStat *rpc.RpcStat   `inter rpc stat`
	rpcCB *rpc.RpcCallBack `inter rpc service callback`
	rpcNode *rpc.RpcNode   `inter rpc node`
	sync.RWMutex
}

//construct (STEP1)
func NewRpcService(ports ...int) *RpcService {
	//set port
	port := define.DefaultRpcPort
	if ports != nil && len(ports) > 0 {
		port = ports[0]
	}

	//init rpc nodes
	rpcNode := rpc.NewRpcNode()

	//self init
	this := &RpcService{
		port:    port,
		address: fmt.Sprintf(":%d", port),
		rpcNode: rpcNode,
		rpcStat: rpc.NewRpcStat(rpcNode),
		rpcCB:   rpc.NewRpcCallBack(rpcNode),
	}
	//inter init
	this.interInit()
	return this
}

//quit
func (r *RpcService) Quit() {
	if r.service != nil {
		r.service.Stop()
	}
	if r.rpcNode != nil {
		r.rpcNode.Quit()
	}
	if r.rpcStat != nil {
		r.rpcStat.Quit()
	}
}

//send stream data to remote client
func (r *RpcService) SendStreamToClient(
				remoteAddr string,
				in *proto.Packet,
			) error {
	//check
	if remoteAddr == "" || in == nil {
		return errors.New("invalid parameter")
	}

	//get client stream
	stream, err := r.rpcNode.GetStream(remoteAddr)
	if err != nil {
		return err
	}

	//send to client
	err = stream.SendMsg(in)
	return err
}

//send stream data to all remote client
func (r *RpcService) SendStreamToAll(
				in *proto.Packet,
			) error {
	//check
	if in == nil {
		return errors.New("invalid parameter")
	}
	err := r.rpcNode.CastToNodes(in)
	return err
}

//get port
func (r *RpcService) GetPort() int {
	return r.port
}

//gen new packet
func (r *RpcService) GenPacket() *proto.Packet {
	return &proto.Packet{}
}

//set callback for stream request (STEP2-1)
func (r *RpcService) SetCBForStream(cb func(addr string, data[]byte)bool) {
	r.rpcCB.SetCBForStream(cb)
}

//set callback for general request (STEP2-2)
func (r *RpcService) SetCBForGeneral(cb func(addr string, data[]byte)([]byte, error)) {
	r.rpcCB.SetCBForGen(cb)
}

//set callback for node down (STEP2-3)
func (r *RpcService) SetCBForClientNodeDown(cb func(remoteAddr string) bool) bool {
	return r.rpcNode.SetCBForNodeDown(cb)
}

//begin service (STEP3)
func (r *RpcService) Start() {
	//begin rpc service
	sf := func(listen net.Listener) {
		err := r.service.Serve(listen)
		if err != nil {
			panic(err)
		}
	}
	go sf(r.listener)
}

//get node face
func (r *RpcService) GetNode() *rpc.RpcNode {
	return r.rpcNode
}

////////////////
//private func
///////////////

//inter init
func (r *RpcService) interInit() {
	//try listen tcp port
	listen, err := net.Listen("tcp", r.address)
	if err != nil {
		panic(err)
	}
	r.listener = listen

	//create rpc server with rpc stat support
	r.service = grpc.NewServer(grpc.StatsHandler(r.rpcStat))

	//register call back
	proto.RegisterPacketServiceServer(r.service, r.rpcCB)
}