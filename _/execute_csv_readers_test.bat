call build_csv_readers_test.bat
if errorlevel 1 goto exit

cd ..
cd bin
start csv_readers_test.exe
cd ..
cd src/ih_server/_

exit