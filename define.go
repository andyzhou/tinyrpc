package tinyrpc

//default value
const (
	DefaultRpcPort = 7100
)

//rpc mode
const (
	ModeOfRpcGen = iota
	ModeOfRpcStream
)

//rpc packet kind
const (
	PacketKindReq = iota
	PacketKindResp
)

//internal macro define
const (
	NodeCheckRate = 5
	NodeDataChanSize = 1024
	MaxTryTimes = 5
)