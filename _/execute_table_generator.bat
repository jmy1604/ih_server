call build_table_generator.bat
if errorlevel 1 goto exit

cd ..
cd bin
start table_generator.exe -f ../conf/table_generator.json
cd ..
cd src/ih_server/_

exit