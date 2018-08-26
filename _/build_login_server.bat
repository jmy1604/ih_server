call build_framework.bat
if errorlevel 1 goto exit

set GOPATH=D:\work\wjxz_server

go build -o ../main/login_server/login_server_server.exe main/login_server
if errorlevel 1 goto exit

go install main/login_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build login_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build login_server ok