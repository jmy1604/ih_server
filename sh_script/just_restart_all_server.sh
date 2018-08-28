set -x

bash ./kill_all_server.sh

sleep 1s

cd ../../../bin
nohup `pwd`/center_server &
sleep 1s
nohup `pwd`/rpc_server &
sleep 1s
nohup `pwd`/hall_server &
sleep 1s 
nohup `pwd`/hall_server -f `pwd`/../run/ih_server/conf/hall_server2.json &
sleep 1s
nohup `pwd`/login_server &
