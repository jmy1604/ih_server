#!/bin/bash
mkdir ih_server
cd ih_server
mkdir bin

cp ../bin/center_server ./bin
cp ../bin/login_server ./bin
cp ../bin/hall_server ./bin
cp ../bin/rpc_server ./bin
cp ../bin/test_client ./bin

cp -r ../conf ./ 
cp -r ../game_data ./
cp -r ../sh_script ./
cd ../

tar -czvf ih_server.tar.gz ih_server
rm -fr ih_server
