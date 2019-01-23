#!/bin/bash
set -x

bash ./kill_all_server.sh

sleep 1s

cd ../bin
nohup `pwd`/center_server 1>/dev/null 2>cs_err.log &
sleep 1s
nohup `pwd`/rpc_server 1>/dev/null 2>rs_err.log &
sleep 1s
nohup `pwd`/hall_server 1>/dev/null 2>hs_err.log &
sleep 1s 
nohup `pwd`/hall_server -f `pwd`/../conf/hall_server2.json 1>/dev/null 2>hs2_err.log &
sleep 1s
nohup `pwd`/login_server 1>/dev/null 2>ls_err.log &
