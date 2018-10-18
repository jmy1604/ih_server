#!/bin/bash
export GOPATH=$(pwd)/../../..
set -x
go install -v -work github.com/gomodule/redigo/internal
go install -v -work github.com/gomodule/redigo/redisx
go install -v -work github.com/gomodule/redigo/redis
go install -v -work ih_server/src/table_config
go install -v -work ih_server/src/rpc_common
go build -i -o ../bin/center_server ih_server/src/center_server
go build -i -o ../bin/login_server ih_server/src/login_server
go build -i -o ../bin/hall_server ih_server/src/hall_server
go build -i -o ../bin/rpc_server ih_server/src/rpc_server
go build -i -o ../bin/test_client ih_server/src/test_client
