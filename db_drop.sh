#!/bin/bash

HOSTNAME="127.0.0.1"
PORT="3306"
USERNAME="root"
PASSWORD=""
DBNAMES=("ih_login_server" "ih_hall_server" "ih_hall_server_2" "ih_hall_server_3" "ih_hall_server_4")

for var in ${DBNAMES[@]};
do
	mysql -h${HOSTNAME} -P${PORT}  -u${USERNAME} -p${PASSWORD} -e "drop database $var"
done
