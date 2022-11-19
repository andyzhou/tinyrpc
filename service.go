package tinyrpc

import (
	"fmt"
	"github.com/andyzhou/tinyrpc/proto"
	"google.golang.org/grpc"
	"net"
)

/*
 * rpc service interface
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 */

//rpc service face
type RpcService struct {
	port int `rpc port`
	address string `rpc service address`
	listener net.Listener `tcp listener`
	service *grpc.Server
	rpcStat *RpcStat `inter rpc stat`
	rpcCB *RpcCallBack `inter rpc service callback`
	rpcNode *RpcNode `inter rpc node`
}

//construct (STEP1)
func NewRpcService(ports ...int) *RpcService {
	//set port
	port := DefaultRpcPort
	if ports != nil && len(ports) > 0 {
		port = ports[0]
	}

	//self init
	rpcNode := NewGRpcNode()
	this := &RpcService{
		port:port,
		address:fmt.Sprintf(":%d", port),
		rpcNode: rpcNode,
		rpcStat: NewGRPCStat(rpcNode),
		rpcCB: NewRpcCallBack(rpcNode),
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
}

//set callback for stream request (STEP2-1)
func (r *RpcService) SetCBForStream(cb func(string,[]byte)bool) {
	r.rpcCB.SetCBForStream(cb)
}

//set callback for general request (STEP2-2)
func (r *RpcService) SetCBForGeneral(cb func(string,[]byte)[]byte) {
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
func (r *RpcService) GetNode() *RpcNode {
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