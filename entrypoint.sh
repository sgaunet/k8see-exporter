#!/usr/bin/env bash

function checkVarIsNotEmpty
{
  var="$1"
  eval "value=\$$var"
  if [ -z "$value" ]
  then  
    echo "ERROR: $var not set. EXIT 1"
    exit 1
  fi
}

checkVarIsNotEmpty REDIS_HOST
checkVarIsNotEmpty REDIS_STREAM

echo "redis_host: $REDIS_HOST"          > /opt/k8see-exporter/conf.yaml
echo "redis_port: ${REDIS_PORT:-6379}" >> /opt/k8see-exporter/conf.yaml
echo "redis_password: $REDIS_PASSWORD" >> /opt/k8see-exporter/conf.yaml
echo "redis_stream: $REDIS_STREAM"     >> /opt/k8see-exporter/conf.yaml

exec $@
