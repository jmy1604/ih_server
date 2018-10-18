call build_framework.bat
if errorlevel 1 goto exit

go build -i -o ../bin/db_server.exe ih_server/src/db_server
if errorlevel 1 goto exit

go install ih_server/src/db_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build db_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build db_server ok