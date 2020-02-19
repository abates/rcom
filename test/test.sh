#!/bin/bash

set -e
OLDDIR=$PWD
cd /home/rcom/devel/rcom/cmd/rcom
./rcom -debug key deploy -a -e /home/rcom/devel/rcom/cmd/rcom/rcom server
./rcom -debug client -e /home/rcom/devel/rcom/cmd/rcom/rcom server lp:rp &
sleep 2 # wait for device to be created
cat lp &
ssh -i ~/.ssh/id_rsa_rcom server
cd $OLDDIR
