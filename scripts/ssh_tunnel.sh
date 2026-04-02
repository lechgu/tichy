#!/bin/bash

# exit on error
set -e

# define remote service ports
llm_port=8180
emb_port=8181
qdr_port=7334

user=$1
node=$2

# validate input
if [[ -z "$user" || -z "$node" ]]; then
  echo "Usage: $0 <user> <remote-node>"
  exit 1
fi

echo "Creating SSH tunnel to $user@$node"
echo "Forwarding:"
echo "  LLM  : localhost:$llm_port -> remote:$llm_port"
echo "  EMB  : localhost:$emb_port -> remote:$emb_port"
echo "  QDR  : localhost:$qdr_port -> remote:$qdr_port"

# run tunnel (no shell)
ssh -N \
  -L ${llm_port}:localhost:${llm_port} \
  -L ${emb_port}:localhost:${emb_port} \
  -L ${qdr_port}:localhost:${qdr_port} \
  ${user}@${node}
