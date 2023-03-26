package rpc

import (
	"context"
	"google.golang.org/grpc/stats"
	"log"
	"sync"
)

/*
 * RPC stat handler for service
 * @author <AndyZhou>
 * @mail <diudiu8848@163.com>
 * Need apply `TagConn`, `TagRPC`, `HandleConn`, `HandleRPC` methods.
 */

//connect ctx key info
type connCtxKey struct{}

//stat face
type RpcStat struct {
	nodeFace *RpcNode
	connMap map[*stats.ConnTagInfo]string
	sync.RWMutex
}

//declare global variable
var RunRpcStat *RpcStat

//construct
func NewRpcStat(nodeFace *RpcNode) *RpcStat {
	this := &RpcStat{
		nodeFace:nodeFace,
		connMap: map[*stats.ConnTagInfo]string{},
	}
	return this
}

//quit
func (h *RpcStat) Quit() {
	h.Lock()
	defer h.Unlock()
	h.connMap = map[*stats.ConnTagInfo]string{}
}

//get connect tag
func (h *RpcStat) GetConnTagFromContext(ctx context.Context) (*stats.ConnTagInfo, bool) {
	tag, ok := ctx.Value(connCtxKey{}).(*stats.ConnTagInfo)
	return tag, ok
}

//cb for rpc api
func (h *RpcStat) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	log.Printf("TagConn, from address:%v\n", info.RemoteAddr)
	return context.WithValue(ctx, connCtxKey{}, info)
}

//cb for rpc api
func (h *RpcStat) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	//log.Printf("TagRPC, method name:%v\n", info.FullMethodName)
	return ctx
}

//cb for rpc api
func (h *RpcStat) HandleConn(ctx context.Context, s stats.ConnStats) {
	//get tag current ctx
	h.Lock()
	defer h.Unlock()
	tag, ok := h.GetConnTagFromContext(ctx)
	if !ok {
		log.Printf("can not get conn tag\n")
		return
	}

	switch s.(type) {
	case *stats.ConnBegin:
		//connMap[tag] = ""
		h.connMap[tag] = ""
		log.Printf("begin conn, tag = (%p)%#v, now connections = %d\n", tag, tag, len(h.connMap))
	case *stats.ConnEnd:
		delete(h.connMap, tag)
		log.Printf("end conn, tag = (%p)%#v, now connections = %d\n", tag, tag, len(h.connMap))
		//run node face to remove end connect
		if h.nodeFace != nil {
			remoteAddr := tag.RemoteAddr.String()
			h.nodeFace.RemoveStream(remoteAddr)
			if h.nodeFace.cbForNodeDown != nil {
				h.nodeFace.cbForNodeDown(remoteAddr)
			}
		}
	default:
		log.Printf("illegal ConnStats type\n")
	}
}

//cb for rpc api
func (h *RpcStat) HandleRPC(ctx context.Context, s stats.RPCStats) {
	//fmt.Println("HandleRPC, IsClient:", s.IsClient())
}
