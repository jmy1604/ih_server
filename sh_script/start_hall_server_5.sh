#!/bin/bash
set -x

cd ../bin
nohup `pwd`/hall_server -f `pwd`/../conf/hall_server5.json 1>/dev/null 2>hs5_err.log &
