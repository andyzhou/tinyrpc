package define

import "time"

// default value
const (
	DefaultRpcPort         = 7100
	DefaultMsgSize         = 1024 * 4 //4KB
	DefaultReadMsgSize     = 1024 * 4    //4KB
	DefaultStreamTimeOut   = 10          //5 minutes
	DefaultConnTimeOut     = 5           //xxx seconds
	DefaultActiveSeconds   = 10          //xxx seconds
	DefaultTimeoutSeconds  = 15          //xxx seconds
	DefaultMaxConnectIdles = 10          //xxx number
)

// rpc mode
const (
	ModeOfRpcGen = iota
	ModeOfRpcStream
	ModeOfRpcAll
)

// internal macro define
const (
	NodeCheckRate    = 5 * time.Second //xx seconds
	NodeDataChanSize = 1024
	MaxTryTimes      = 3
)

// error code
const (
	ErrCodeOfSucceed = iota
	ErrCodeOfInvalidPara
	ErrCodeOfInterError
	ErrCodeOfNodeDown
	ErrCodeOfNoSuchData
	ErrCodeOfNoCallBack
	ErrCodeOfRunError
)
