call build_framework.bat
if errorlevel 1 goto exit

go install ih_server/src/center_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build center_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build center_server ok