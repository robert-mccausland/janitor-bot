#!/bin/bash

set -e
shopt -s expand_aliases

alias docker-compose='docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$PWD:$PWD" \
    -w="$PWD" \
    docker/compose:1.27.4'

cd /home/apps/janitor-bot

docker-compose down
docker-compose up -d --force-recreate
