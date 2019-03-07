call set_go_path.bat
call build_table_config.bat

go install ih_server/src/rpc_proto
go install ih_server/src/rpc_server
if errorlevel 1 goto exit

go build -i -o ../bin/rpc_server.exe ih_server/src/rpc_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build rpc_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build rpc_server ok