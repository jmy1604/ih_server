call set_go_path.bat
call build_framework.bat
if errorlevel 1 goto exit

go build -i -o ../bin/center_server.exe ih_server/src/center_server
if errorlevel 1 goto exit_center

go build -i -o ../bin/login_server.exe ih_server/src/login_server
if errorlevel 1 goto exit_login

go build -i -o ../bin/hall_server.exe ih_server/src/hall_server
if errorlevel 1 goto exit_hall

go build -i -o ../bin/rpc_server.exe ih_server/src/rpc_server
if errorlevel 1 goto exit_rpc

go build -i -o ../bin/test_client.exe ih_server/src/test_client
if errorlevel 1 goto exit_test

if errorlevel 0 goto ok

:exit_center
echo build center_server failed !!!

:exit_login
echo build login_server failed !!!

:exit_hall
echo build hall_server failed !!!

:exit_rpc
echo build rpc_server failed !!!

:exit_test
echo build test_client failed !!!

:ok
echo build all ok