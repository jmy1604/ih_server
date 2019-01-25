cd../proto

md gen_go
cd gen_go

md server_message
md db_center
md db_hall

cd ../../third_party/protobuf

move protoc.exe ../../proto
call set_go_path.bat

move protoc-gen-go.exe ../../proto

cd ../../proto
protoc.exe --go_out=./gen_go/server_message/ server_message.proto
cd ../_
if errorlevel 1 goto exit

cd ../proto
go install ih_server/proto/gen_go/server_message
cd ../_
if errorlevel 1 goto exit

cd ../proto
protoc.exe --go_out=./gen_go/db_hall/ db_hallsvr.proto
cd ../_
if errorlevel 1 goto exit

cd ../proto
protoc.exe --go_out=./gen_go/db_login/ db_loginsvr.proto
cd ../_
if errorlevel 1 goto exit

cd ../proto
protoc.exe --go_out=./gen_go/db_rpc/ db_rpcsvr.proto
cd ../_
if errorlevel 1 goto exit

cd ../proto
move protoc.exe ../third_party/protobuf
move protoc-gen-go.exe ../third_party/protobuf
cd ../_

goto ok

:exit
echo gen message failed!!!!!!!!!!!!!!!!!!!!!!!!!!!!

:ok
echo gen message ok