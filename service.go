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

//global variable
var (
	_service *Service
	_serviceOnce sync.Once
)

//service para
type ServicePara struct {
	Port int //rpc port
	MaxMsgSize int //max message size, default 4MB
}

//rpc service face
type Service struct {
	para *ServicePara //rpc service para
	address string //rpc service address
	listener net.Listener //tcp listener
	service *grpc.Server
	rpcStat *rpc.Stat   //inter rpc stat
	rpcCB *rpc.CallBack //inter rpc service callback
	rpcNode *rpc.Node   //inter rpc node
	started bool
	sync.RWMutex
}

//get single instance
func GetService(paras ...*ServicePara) *Service {
	_serviceOnce.Do(func() {
		_service = NewService(paras...)
	})
	return _service
}

//construct (STEP1)
func NewService(paras ...*ServicePara) *Service {
	var (
		para *ServicePara
	)

	//detect
	if paras != nil && len(paras) > 0 {
		para = paras[0]
	}else{
		//init default para
		para = &ServicePara{
			Port: define.DefaultRpcPort,
			MaxMsgSize: 1024 * 1024 * 4, //4MB
		}
	}

	//set default port
	if para.Port <= 0 {
		para.Port = define.DefaultRpcPort
	}

	//init rpc nodes
	rpcNode := rpc.NewNode()

	//self init
	this := &Service{
		para: para,
		address: fmt.Sprintf(":%d", para.Port),
		rpcNode: rpcNode,
		rpcStat: rpc.NewStat(rpcNode),
		rpcCB:   rpc.NewCallBack(rpcNode),
	}
	//inter init
	this.interInit()
	return this
}

//quit
func (r *Service) Quit() {
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
func (r *Service) SendStreamData(
				in *proto.Packet,
				remoteAddress ...string,
			) error {
	var (
		err error
	)
	//check
	if in == nil {
		return errors.New("invalid parameter")
	}

	//has remote address
	if remoteAddress != nil && len(remoteAddress) > 0 {
		//loop process
		for _, remoteAddr := range remoteAddress {
			//get client stream
			stream, subErr := r.rpcNode.GetStream(remoteAddr)
			if subErr != nil {
				return subErr
			}
			//send to client
			subErr = stream.SendMsg(in)
			if subErr != nil {
				return subErr
			}
		}
	}else{
		//cast to all nodes
		err = r.rpcNode.CastToNodes(in)
	}
	return err
}

//set callback for stream request (STEP2-1)
func (r *Service) SetCBForStream(
			cb func(addr string, in *proto.Packet)error) {
	r.rpcCB.SetCBForStream(cb)
}

//set callback for general request (STEP2-2)
func (r *Service) SetCBForGeneral(
			cb func(addr string, in *proto.Packet)(*proto.Packet, error)) {
	r.rpcCB.SetCBForGen(cb)
}

//set callback for node down (STEP2-3)
func (r *Service) SetCBForClientNodeDown(
			cb func(remoteAddr string) bool) bool {
	return r.rpcNode.SetCBForNodeDown(cb)
}

//begin service (STEP3)
//support assigned rpc service port
func (r *Service) Start(ports ...int) error {
	var (
		port int
	)
	//check and set port
	if ports != nil && len(ports) > 0 {
		port = ports[0]
	}
	if port > 0 {
		r.para.Port = port
	}
	if r.started {
		return errors.New("service had started")
	}

	//re-set address
	r.address = fmt.Sprintf(":%d", r.para.Port)

	//try listen tcp port
	listen, err := net.Listen("tcp", r.address)
	if err != nil {
		return err
	}

	//sync listener
	r.listener = listen
	r.started = true

	//begin rpc service
	sf := func(listen net.Listener) {
		err = r.service.Serve(listen)
		if err != nil {
			panic(any(err))
		}
	}
	go sf(r.listener)
	return err
}

//get port
func (r *Service) GetPort() int {
	return r.para.Port
}

//get node face
func (r *Service) GetNode() *rpc.Node {
	return r.rpcNode
}

//gen new packet
func (r *Service) GenPacket() *proto.Packet {
	return &proto.Packet{}
}

////////////////
//private func
///////////////

//inter init
func (r *Service) interInit() {
	//create rpc server with max msg size and rpc stat support
	r.service = grpc.NewServer(
			grpc.MaxSendMsgSize(r.para.MaxMsgSize), //max send size
			grpc.MaxRecvMsgSize(r.para.MaxMsgSize), //max receive size
			grpc.StatsHandler(r.rpcStat), //rpc stat
		)

	//register call back
	proto.RegisterPacketServiceServer(r.service, r.rpcCB)
}