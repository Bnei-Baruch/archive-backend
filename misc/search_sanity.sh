#!/usr/bin/env bash

set +e
set -x

BASE_DIR="/sites/archive-backend"
TIMESTAMP="$(date '+%Y%m%d%H%M%S')"
LOG_FILE="$BASE_DIR/logs/es/eval_sanity_$TIMESTAMP.log"
FLAT_REPORT_FILE="$BASE_DIR/logs/es/eval_sanity_$TIMESTAMP.flat.report"

cd ${BASE_DIR}
./archive-backend eval --server=https://kabbalahmedia.info/backend --flat_report=${FLAT_REPORT_FILE} --eval_set=./search/data/sanity.csv >> ${LOG_FILE} 2>&1

SANITY_OK="$(egrep -c "Good.*100.00%" ${LOG_FILE})"

# Cleanup old logs (older then week).
find ${BASE_DIR}/logs/es -name "eval_sanity_*.log" -type f -mtime +1 -exec rm -f {} \;

if [ "${SANITY_OK}" = "1" ];then
        echo "Sanity OK."
        exit 0
else
        echo "Sanity failed." | mail -s "ERROR: Search sanity." -r "mdb@bbdomain.org" -a ${LOG_FILE} -a ${FLAT_REPORT_FILE} edoshor@gmail.com kolmanv@gmail.com
    exit 1
fi
