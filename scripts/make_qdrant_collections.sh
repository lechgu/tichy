#!/bin/bash
set -e

adir=/mnt/data1/vk/AI
sdir=$adir/docs
export TICHY_ENV=$adir/tichy/.env

# go to project root (where .env is)
cd $adir/tichy

# load env
set -a
source .env
set +a

collections=(
  "CHAP"
  "FOXDEN"
  "Computing"
  "SOP"
  "CHESS_elog"
  "CESR_Ops"
)

for c in "${collections[@]}"; do
    echo "==== $(date) | Collection: $sdir/$c ===="
    echo "time QDRANT_COLLECTION=$c $adir/tichy/tichy ingest -m text -s $sdir/$c"
    time QDRANT_COLLECTION=$c $adir/tichy/tichy ingest -m text -s $sdir/$c
done
