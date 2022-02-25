#!/bin/sh
secretkey=$(cat secretkey.txt)
nohup ./miner-proxy -l :12345 -r xxxxx:123456 -sc -k $secretkey -debug >> miner.log 2>& 1 &
