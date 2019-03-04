package rpc_common

type H2H_GetRequest struct {
	PlayerId int32
	ArgData  []byte
}

type H2H_GetResponse struct {
	ResultData []byte
}

type H2H_MultiGetRequest struct {
	PlayerIds []int32
	ArgData   []byte
}

type H2H_MultiGetResponse struct {
	ResultData []byte
}

type H2H_BroadcastRequest struct {
	PlayerIds []int32
	ArgData   []byte
}

type H2H_BroadcastResponse struct {
	ResultData []byte
}
