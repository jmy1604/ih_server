call gen_server_message.bat
if errorlevel 1 goto exit

call gen_client_message.bat
if errorlevel 1 goto exit

go install ih_server/libs/log
if errorlevel 1 goto exit

go install ih_server/libs/timer
if errorlevel 1 goto exit

go install ih_server/libs/perf
if errorlevel 1 goto exit

go install ih_server/libs/socket
if errorlevel 1 goto exit

go install ih_server/libs/server_conn
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build framework failed!!!!!!!!!!!!!!!!!!

:ok
echo build framework ok