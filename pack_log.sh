#!/bin/bash

if [ "$1" = "" ];then
    echo "err: please input a log file directory"
    exit
fi

old_pwd=$(pwd)
log_dir=$1
cd $log_dir

res=$(find ./ -type f -printf "%AD %AT %p\n" | grep ".txt$" | awk 'NR==1{print $3}')

if [ $res = "" ];then
    echo "warn: not found log file in directory $log_dir"
    exit
fi

echo "find the file: $res"
log_file=${res##*\/}

tar -czvf $log_file.tar.gz $log_file
rm $log_file
cd $old_pwd

echo "done"
