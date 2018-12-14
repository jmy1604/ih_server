call set_go_path.bat

cd../proto

md gen_go
cd gen_go

md client_message

cd ../../third_party/protobuf

move protoc.exe ../../proto
move protoc-gen-go.exe ../../proto

cd ../../proto
protoc.exe --go_out=./gen_go/client_message/ client_message.proto
protoc.exe --go_out=./gen_go/client_message_id/ client_message_id.proto
cd ../_
if errorlevel 1 goto exit

cd ../proto
go install ih_server/proto/gen_go/client_message
go install ih_server/proto/gen_go/client_message_id
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