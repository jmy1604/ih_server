#!/bin/bash
set -x

nohup `pwd`/hall_server -f `pwd`/../conf/hall_server4.json &
