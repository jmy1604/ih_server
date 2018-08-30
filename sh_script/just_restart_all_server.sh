set -x

sh ./kill_all_server.sh

sleep 1s

cd ../../../src/ih_server
sh ./copy_game_data.sh

cd ../../bin
nohup `pwd`/center_server &
sleep 1s
nohup `pwd`/rpc_server &
sleep 1s
nohup `pwd`/hall_server &
sleep 1s 
nohup `pwd`/hall_server -f `pwd`/../run/ih_server/conf/hall_server2.json &
sleep 1s
nohup `pwd`/login_server &
