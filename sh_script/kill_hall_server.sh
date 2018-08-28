cd ../../../bin
export cur_dir=`pwd`
export cur_server_id=`ps aux | grep hall_server | grep $cur_dir | awk 'NR==1{print $2}'`
if [ -z $cur_server_id ] ; then
	echo "cur_hall_server not running"
	exit 0
else
	echo "cur_hall_server id is $cur_server_id"
fi

kill -15 $cur_server_id

while [[ $cur_server_id != "" ]]
do
	export cur_server_id=`ps aux | grep hall_server | grep $cur_dir | awk 'NR==1{print $2}'`
	if [ -z $cur_server_id ] ; then
        	echo "close_hall_server ok"
	else
		kill -15 $cur_server_id
        	echo "wait hall_server closing"
	fi

	sleep 1s
done
