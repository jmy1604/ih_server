go build -o ../main/test_client/test_client.exe main/test_client
if errorlevel 1 goto exit

go install main/test_client
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build test_client failed!!!!!!!!!!!!!!!!!!!

:ok
echo build test_client ok