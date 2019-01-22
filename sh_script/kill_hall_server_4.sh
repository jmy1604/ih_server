#!/bin/bash

cd ../bin
export cur_dir=`pwd`

export cur_server_id=`ps aux | grep hall_server | grep hall_server4.json | grep $cur_dir | awk 'NR==1{print $2}'`
if [ -z $cur_server_id ] ; then
	echo "hall_server_4 not running"
	exit 0
else
	echo "hall_server_4 id is $cur_server_id"
fi

kill -15 $cur_server_id

while [[ $cur_server_id != "" ]]
do
	export cur_server_id=`ps aux | grep hall_server  | grep hall_server4.json | grep $cur_dir | awk 'NR==1{print $2}'`
	if [ -z $cur_server_id ] ; then
        	echo "close hall_server_4 ok"
	else
		kill -15 $cur_server_id
        	echo "wait hall_server_4 closing"
	fi

	sleep 1s
done
