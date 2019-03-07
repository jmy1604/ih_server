call set_go_path.bat
call build_framework.bat
if errorlevel 1 goto exit

call build_table_config.bat
if errorlevel 1 goto exit

go install ih_server/src/rpc_proto
go install ih_server/src/hall_server
if errorlevel 1 goto exit

go build -i -o ../bin/hall_server.exe ih_server/src/hall_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build hall_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build hall_server ok