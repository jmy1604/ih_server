call set_go_path.bat

go install ih_server/src/csv_readers_test
if errorlevel 1 goto exit

go build -i -o ../bin/csv_readers_test.exe ih_server/src/csv_readers_test
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build csv_readers_test failed!!!!!!!!!!!!!!!!!!!

:ok
echo build csv_readers_test ok