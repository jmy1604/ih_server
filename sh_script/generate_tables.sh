#!/bin/bash
export GOPATH=$(pwd)/../../..
set -x
go build -i -o ../bin/table_generator ih_server/src/table_generator
cd ../bin
./table_generator -f ../conf/table_generator.json
cd ../sh_script
