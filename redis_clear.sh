#!/bin/bash

REDIS_HOSTNAME="127.0.0.1"
REDIS_PORT="6379"

redis-cli -h $REDIS_HOSTNAME -p $REDIS_PORT del "ih:hall_server:google_pay" "ih:hall_server:apple_pay" "ih:share_data:uid_player_list" "ih:hall_server:uid_token_key"
