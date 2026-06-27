#!/bin/bash

set -e
shopt -s expand_aliases

alias docker-compose='docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$PWD:$PWD" \
    -w="$PWD" \
    docker/compose:1.27.4'

cd /home/apps/janitor-bot


# Hack to fix volume permissions because docker-compose doesn't support setting the user for volumes
docker volume create --driver=local janitor-bot_janitor-bot-data
docker run --rm -v janitor-bot_janitor-bot-data:/var/lib/janitor-bot/data \
    alpine:3.22 sh -c "mkdir -p /var/lib/janitor-bot/data && chown -R 1000:1000 /var/lib/janitor-bot/data"

docker-compose down
docker-compose up -d --force-recreate
