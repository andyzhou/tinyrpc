package rpc

import (
	"context"
	"errors"
	"google.golang.org/grpc/stats"
	"log"
	"runtime"
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
type Stat struct {
	nodeFace *Node
	connMap map[*stats.ConnTagInfo]string
	//relate cb
	cbForRemoteUp func(string) error
	cbForRemoteDown func(string) error
	sync.RWMutex
}

//declare global variable
var RunStat *Stat

//construct
func NewStat(nodeFace *Node) *Stat {
	this := &Stat{
		nodeFace:nodeFace,
		connMap: map[*stats.ConnTagInfo]string{},
	}
	return this
}

//quit
func (h *Stat) Quit() {
	//clear connect map
	h.Lock()
	defer h.Unlock()
	for k, _ := range h.connMap {
		delete(h.connMap, k)
	}
	h.connMap = map[*stats.ConnTagInfo]string{}

	//gc memory
	runtime.GC()
}

//inter cb setup
func (h *Stat) SetCBForRemoteUp(cb func(string)error) error {
	if cb == nil {
		return errors.New("invalid parameter")
	}
	h.cbForRemoteUp = cb
	return nil
}

func (h *Stat) SetCBForRemoteDown(cb func(string)error) error {
	if cb == nil {
		return errors.New("invalid parameter")
	}
	h.cbForRemoteDown = cb
	return nil
}

/////////////////////
//apply of interface
/////////////////////

//get connect tag
func (h *Stat) GetConnTagFromContext(ctx context.Context) (*stats.ConnTagInfo, bool) {
	tag, ok := ctx.Value(connCtxKey{}).(*stats.ConnTagInfo)
	return tag, ok
}

//cb for rpc api
//remote address connected
func (h *Stat) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	//log.Printf("TagConn, from address:%v\n", info.RemoteAddr)
	if h.cbForRemoteUp != nil {
		h.cbForRemoteUp(info.RemoteAddr.String())
	}
	return context.WithValue(ctx, connCtxKey{}, info)
}

//cb for rpc api
func (h *Stat) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	//log.Printf("TagRPC, method name:%v\n", info.FullMethodName)
	return ctx
}

//cb for rpc api
func (h *Stat) HandleConn(ctx context.Context, s stats.ConnStats) {
	//get tag current ctx with lock
	h.Lock()
	defer h.Unlock()
	tag, ok := h.GetConnTagFromContext(ctx)
	if !ok || tag == nil {
		log.Printf("rpcStat:HandleConn, can not get conn tag\n")
		return
	}

	//process stats
	switch s.(type) {
	case *stats.ConnBegin:
		{
			h.connMap[tag] = ""
			//log.Printf("begin conn, tag = (%p)%#v, now connections = %d\n", tag, tag, len(h.connMap))
		}
	case *stats.ConnEnd:
		{
			//remove connect disconnect or ended
			//call cb for connect end
			if h.cbForRemoteDown != nil {
				h.cbForRemoteDown(tag.RemoteAddr.String())
			}

			//delete tag from connect map
			delete(h.connMap, tag)

			//log.Printf("end conn, tag = (%p)%#v, now connections = %d\n", tag, tag, len(h.connMap))
			//run node face to remove end connect
			if h.nodeFace != nil {
				remoteAddr := tag.RemoteAddr.String()
				h.nodeFace.RemoveStream(remoteAddr)
			}
		}
	default:
		log.Printf("illegal ConnStats type\n")
	}
}

//cb for rpc api
func (h *Stat) HandleRPC(ctx context.Context, s stats.RPCStats) {
	//fmt.Println("HandleRPC, IsClient:", s.IsClient())
}
