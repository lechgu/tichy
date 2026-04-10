#!/bin/bash
set -e

llm_port=${LLM_PORT:-8180}
emb_port=${EMB_PORT:-8181}
qdr_port=${QDR_PORT:-7334}
tichy_port=${TICHY_PORT:-7070}

user=$1
node=$2
mode=${3:-"--tunnel"}  # default mode

if [[ -z "$user" || -z "$node" ]]; then
  echo "Usage: $0 <user> <remote-node> [--shell]"
  exit 1
fi

echo "SSH tunnel → $user@$node"
echo "LLM  : localhost:$llm_port"
echo "EMB  : localhost:$emb_port"
echo "QDR  : localhost:$qdr_port"
echo "TICHY: localhost:$tichy_port"

# base ssh command
SSH_CMD=(
  ssh
  -L ${llm_port}:localhost:${llm_port}
  -L ${emb_port}:localhost:${emb_port}
  -L ${qdr_port}:localhost:${qdr_port}
  -L ${tichy_port}:localhost:${tichy_port}
)

if [[ "$mode" == "--shell" ]]; then
  echo "Mode: tunnel + interactive shell"
  exec "${SSH_CMD[@]}" ${user}@${node}
else
  echo "Mode: tunnel only (-N)"
  exec "${SSH_CMD[@]}" -N ${user}@${node}
fi
