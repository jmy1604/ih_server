call set_go_path.bat

go install ih_server/src/table_generator
if errorlevel 1 goto exit

go build -i -o ../bin/table_generator.exe ih_server/src/table_generator
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build table_generator failed!!!!!!!!!!!!!!!!!!!

:ok
echo build table_generator ok