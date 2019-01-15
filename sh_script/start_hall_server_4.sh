#!/bin/bash
set -x

cd ../bin
nohup `pwd`/hall_server -f `pwd`/../conf/hall_server4.json &
