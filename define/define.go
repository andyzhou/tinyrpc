package define

import "time"

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

//internal macro define
const (
	NodeCheckRate = 5 * time.Second //xx seconds
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