#!/bin/bash
set -x

cd ../bin
nohup `pwd`/hall_server -f `pwd`/../conf/hall_server2.json 1>/dev/null 2>/dev/null &
