#!/usr/bin/env bash

set +e
set -x

BASE_DIR="/sites/archive-backend"
TIMESTAMP="$(date '+%Y%m%d%H%M%S')"
LOG_FILE="$BASE_DIR/logs/es/reindex_$TIMESTAMP.log"


cd ${BASE_DIR}

supervisorctl stop events

./archive-backend index >> ${LOG_FILE} 2>&1

curl -X POST "elastic.mdb.local:9200/_refresh"

supervisorctl start events


WARNINGS="$(egrep -c "level=(warning|error)" ${LOG_FILE})"

if [ "$WARNINGS" != "0" ];then
	echo "Errors in periodic import of storage catalog to MDB" | mail -s "ERROR: ES reindex" -r "mdb@bbdomain.org" -a ${LOG_FILE} edoshor@gmail.com kolmanv@gmail.com
fi

# Cleanup old logs (older then week).
find ${BASE_DIR}/logs/es -name "reindex_*.log" -type f -mtime +7 -exec rm -f {} \;

if [ "$WARNINGS" != "0" ];then
    echo "Errors or Warnings found."
    exit 1
else 
 	echo "No warnings"
	exit 0
fi

