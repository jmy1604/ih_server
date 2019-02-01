#!/bin/bash

log_dirs =("center_server" "ih_login_server" "rpc_server" "ih_hall_server" "ih_hall_server_2" "ih_hall_server_3" "ih_hall_server_4" "ih_hall_server_5")

mkdir -p logs 
cd bin/logs

for var in ${log_dirs[@]};
do
    cd $var
    find ./ -type f -printf "%AD %AT %p\n" | sort -k1.8n -k1.1nr -k1 | awk 'NR==1{print $1}'
done

mkdir conf
cp -r ../conf/template ./conf 
cp -r ../game_data ./
cp -r ../sh_script ./

cp ../*.sh ./

cd ../

tar -czvf ih_server.tar.gz ih_server
rm -fr ih_server

