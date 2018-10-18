#!/bin/bash
cd ../bin
export cur_dir=`pwd`
export cur_server_id=`ps aux | grep rpc_server | grep $cur_dir | awk '{print $2}'`
if [ -z $cur_server_id ] ; then
	echo "cur_rpc_server not running"
	exit 0
else
	echo "cur_rpc_server id is $cur_server_id"
fi

kill -15 $cur_server_id

while [[ $cur_server_id != "" ]]
do
	export cur_server_id=`ps aux | grep rpc_server | grep $cur_dir | awk '{print $2}'`
	if [ -z $cur_server_id ] ; then
        	echo "close_rpc_server ok"
	else
        	echo "wait rpc_server closing"
	fi

	sleep 1s
done
