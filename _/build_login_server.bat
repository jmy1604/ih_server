call build_framework.bat
if errorlevel 1 goto exit

go install ih_server/src/login_server
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build login_server failed!!!!!!!!!!!!!!!!!!!

:ok
echo build login_server ok