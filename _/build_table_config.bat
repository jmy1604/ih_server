go install main/table_config
if errorlevel 1 goto exit

if errorlevel 0 goto ok

:exit
echo build table_config failed!!!!!!!!!!!!!!!!!!!

:ok
echo build table_config ok