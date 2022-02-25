#!/bin/sh
secretkey=$(cat secretkey.txt)
./miner-proxy -install -l :123 -r baidu.com:123456 -sc -k $secretkey -d && ./miner-proxy -start && ./miner-proxy -stat
