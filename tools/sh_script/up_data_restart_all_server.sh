set -x
sh ./kill_all_server.sh

sleep 5s

cd ../../../bin
nohup `pwd`/center_server &
sleep 1s
nohup `pwd`/rpc_server &
sleep 1s
nohup `pwd`/hall_server &
sleep 1s 
nohup `pwd`/hall_server -f `pwd`/../run/ih_server/conf/hall_server2.cfg &
sleep 1s
nohup `pwd`/login_server &


