package rpc_common

type ServerResponseData struct {
	ResultData []byte
	ErrorCode  int32
}

type G2G_GetRequest struct {
	FromPlayerId int32
	ToPlayerId   int32
	MsgId        int32
	MsgData      []byte
}

type G2G_GetResponse struct {
	Data ServerResponseData
}

type G2G_MultiGetRequest struct {
	FromPlayerId int32
	ToPlayerIds  []int32
	MsgId        int32
	MsgData      []byte
}

type G2G_MultiGetResponse struct {
	Datas []*ServerResponseData
}

type G2G_DataNotify struct {
	FromPlayerId int32
	ToPlayerId   []int32
	MsgId        int32
	MsgData      []byte
}

type G2G_DataNotifyResult struct {
	ErrorCode int32
}

type G2G_MultiDataNotify struct {
	FromPlayerId int32
	ToPlayerIds  []int32
	MsgId        int32
	MsgData      []byte
}

type G2G_MultiDataNotifyResult struct {
	ErrorCodes []int32
}
