package tinyrpc

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tinyrpc/define"
	"github.com/andyzhou/tinyrpc/proto"
	"github.com/andyzhou/tinyrpc/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"net"
	"sync"
	"time"
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
	//base setup
	Host        string //rpc host
	Port        int    //rpc port
	MaxMsgSize  int    //max message size, default 4MB
	MaxReadSize int

	//for keep active
	ActiveSeconds   int
	TimeoutSeconds  int
	MaxConnectIdles int
}

//rpc service face
type Service struct {
	para     *ServicePara  //rpc service para
	address  string        //rpc service address
	listener net.Listener  //tcp listener
	service  *grpc.Server  //inter rpc server
	rpcStat  *rpc.Stat     //inter rpc stat
	rpcCB    *rpc.CallBack //inter rpc service callback
	rpcNode  *rpc.Node     //inter rpc node
	started  bool
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
	}
	if para == nil {
		//init default para
		para = &ServicePara{
			Port: define.DefaultRpcPort,
			MaxMsgSize: define.DefaultMsgSize,
			MaxReadSize: define.DefaultReadMsgSize,
		}
	}

	//set default port
	if para.Port <= 0 {
		para.Port = define.DefaultRpcPort
	}
	if para.MaxMsgSize <= 0 {
		para.MaxMsgSize = define.DefaultMsgSize
	}
	if para.MaxReadSize <= 0 {
		para.MaxReadSize = define.DefaultReadMsgSize
	}

	//init rpc nodes
	rpcNode := rpc.NewNode()

	//self init
	this := &Service{
		para: para,
		address: fmt.Sprintf("%v:%d", para.Host, para.Port),
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
	cb func(remoteAddr string) error) error {
	return r.rpcStat.SetCBForRemoteDown(cb)
}

//set callback for node up to service (STEP2-4)
func (r *Service) SetCBForClientNodeUp(
	cb func(remoteAddr string) error) error {
	return r.rpcStat.SetCBForRemoteUp(cb)
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
	//active seconds
	activeSeconds := r.para.ActiveSeconds
	if activeSeconds <= 0 {
		activeSeconds = define.DefaultActiveSeconds
	}

	//timeout seconds
	timeoutSeconds := r.para.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = define.DefaultTimeoutSeconds
	}

	//max connect idles
	maxConnIdles := r.para.MaxConnectIdles
	if maxConnIdles <= 0 {
		maxConnIdles = define.DefaultMaxConnectIdles
	}

	//keep alive args
	keepAliveArgs := keepalive.ServerParameters{
		Time:              time.Duration(activeSeconds) * time.Second,
		Timeout:           time.Duration(timeoutSeconds) * time.Second,
		MaxConnectionIdle: time.Duration(maxConnIdles) * time.Minute,
	}

	//create rpc server with max msg size and rpc stat support
	r.service = grpc.NewServer(
			grpc.KeepaliveParams(keepAliveArgs),
			grpc.MaxSendMsgSize(r.para.MaxMsgSize), //max send size
			grpc.MaxRecvMsgSize(r.para.MaxMsgSize), //max receive size
			grpc.ReadBufferSize(r.para.MaxReadSize),
			grpc.WriteBufferSize(r.para.MaxReadSize),
			grpc.StatsHandler(r.rpcStat), //rpc stat
		)

	//register service call back
	proto.RegisterPacketServiceServer(r.service, r.rpcCB)
}