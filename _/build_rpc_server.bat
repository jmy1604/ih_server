call build_table_config.bat
go install main/rpc_common
go install main/rpc_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build rpc_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build rpc_server ok