package tinyrpc

//default value
const (
	DefaultRpcPort = 7100
)

//rpc mode
const (
	ModeOfRpcGen = iota
	ModeOfRpcStream
	ModeOfRpcAll
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
	MaxTryTimes = 3
)

//error code
const (
	ErrCodeOfSucceed = iota
	ErrCodeOfInvalidPara
	ErrCodeOfInterError
	ErrCodeOfNodeDown
	ErrCodeOfNoSuchData
	ErrCodeOfNoCallBack
	ErrCodeOfRunError
)