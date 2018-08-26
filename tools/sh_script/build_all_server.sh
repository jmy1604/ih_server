export GOPATH=/root/wjxz_server
set -x
svn up
go install -v -work github.com/gomodule/redigo/internal
go install -v -work github.com/gomodule/redigo/redisx
go install -v -work github.com/gomodule/redigo/redis
go install -v -work main/table_config
go install -v -work main/rpc_common
go install -v -work main/center_server
go install -v -work main/login_server
go install -v -work main/hall_server
go install -v -work main/rpc_server
