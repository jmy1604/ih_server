#!/bin/bash
export GOPATH=$(pwd)/../../..
set -x
go build -i -o ../bin/center_server ih_server/src/center_server
go build -i -o ../bin/login_server ih_server/src/login_server
go build -i -o ../bin/hall_server ih_server/src/hall_server
go build -i -o ../bin/rpc_server ih_server/src/rpc_server
go build -i -o ../bin/test_client ih_server/src/test_client
