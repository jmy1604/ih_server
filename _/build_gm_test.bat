call set_go_path.bat
go install ih_server/src/gm_test
if errorlevel 1 goto exit

go build -i -o ../bin/gm_test.exe ih_server/src/gm_test
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build gm_test failed!!!!!!!!!!!!!!!!!!!

:ok
echo build gm_test ok