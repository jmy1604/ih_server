#!/bin/bash
set -x

cd ../bin
nohup env GOTRACEBACK=crash `pwd`/hall_server -f `pwd`/../conf/hall_server3.json 1>/dev/null 2>>hs3_err.log &
