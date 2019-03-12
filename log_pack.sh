#!/bin/bash

if [ "$1" = "" ];then
    echo "err: please input a log file directory"
    exit
fi

old_pwd=$(pwd)
log_dir=$1
cd $log_dir

res=$(ls ./ -alrt | grep ".txt$" | awk 'NR==1')

echo $res
if [ -z "$res" ];then
    echo "warn: not found log file in directory $log_dir"
    exit
fi

log_file=${res##*\ }
echo "find the file: $log_file"

tar -czvf $log_file.tar.gz $log_file
rm $log_file
cd $old_pwd

echo "done"
