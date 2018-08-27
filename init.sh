#!/bin/sh

mkdir -p ../../run
cd ../../run
mkdir -p ih_server
cd ih_server
mkdir -p conf

cd ../../src/ih_server
cp -r conf/template/* ../../run/ih_server/conf
cp -r tools/sh_script ../../run/ih_server
