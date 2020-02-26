#!/usr/bin/env bash
set +e
set -x

echo "delete previous snapshot"
curl -XDELETE 'http://localhost:9200/_snapshot/backup/full'

echo "start a new snapshot"
curl -XPUT 'http://localhost:9200/_snapshot/backup/full' -H 'Content-Type: application/json' -d '{
      "indices": "_all",
      "ignore_unavailable": true,"include_global_state": false
  }'
