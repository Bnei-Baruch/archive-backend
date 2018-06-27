#!/usr/bin/env bash

set +e
set -x

BASE_DIR="/sites/archive-backend"
TIMESTAMP="$(date '+%Y%m%d%H%M%S')"
LOG_FILE="$BASE_DIR/logs/es/reindex_$TIMESTAMP.log"

cd ${BASE_DIR}
./archive-backend convert-docx >> ${LOG_FILE} 2>&1

supervisorctl stop events

./archive-backend index classifications >> ${LOG_FILE} 2>&1
./archive-backend index collections >> ${LOG_FILE} 2>&1
./archive-backend index sources >> ${LOG_FILE} 2>&1
./archive-backend index units >> ${LOG_FILE} 2>&1

curl -X POST "elastic.mdb.local:9200/_refresh"

supervisorctl start events

WARNINGS="$(egrep -c "level=(warning|error)" ${LOG_FILE})"

if [ "$WARNINGS" = "" ];then
        echo "No warnings"
        exit 0
fi

echo "Errors in periodic import of storage catalog to MDB" | mail -s "ERROR: ES reindex" -r "mdb@bbdomain.org" -a ${LOG_FILE} edoshor@gmail.com

find ${BASE_DIR}/logs/es -name "reindex_*.log" -type f -mtime +7 -exec rm -f {} \;
