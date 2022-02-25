#!/bin/sh
cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n1 > secretkey.txt
