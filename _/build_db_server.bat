call build_framework.bat
if errorlevel 1 goto exit

go build -o ../youma/db_server/db_server_server.exe youma/db_server
if errorlevel 1 goto exit

go install youma/db_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build db_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build db_server ok