#!/bin/sh
mkdir ih_server
cd ih_server
mkdir bin

cp ../../../bin/center_server ./bin
cp ../../../bin/login_server ./bin
cp ../../../bin/hall_server ./bin
cp ../../../bin/rpc_server ./bin
cp ../../../bin/test_client ./bin

mkdir run
cd run
mkdir ih_server

cd ../
cp -r ../../../run/ih_server/conf run/ih_server
cp -r ../../../run/ih_server/game_data run/ih_server
cp -r ../../../run/ih_server/sh_script run/ih_server
cd ../

tar -czvf ih_server.tar.gz ih_server
rm -fr ih_server
