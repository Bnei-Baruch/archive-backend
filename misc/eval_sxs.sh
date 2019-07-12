#!/usr/bin/env bash

set +e
set -x

BASE_DIR="/sites/archive-backend"
RECALL_SET_DIR="${BASE_DIR}/search/data"
LOGS_DIR="${BASE_DIR}/logs/es"
GOLDEN_DIR="${BASE_DIR}/search/golden"
BACKEND="https://kabbalahmedia.info/backend"

# LOCAL
BASE_DIR="."
RECALL_SET_DIR="${BASE_DIR}/search/data"
LOGS_DIR="/tmp/logs/es"

TIMESTAMP="$(date '+%Y%m%d%H%M%S')"

BASE_LOG_FILE_HE="${LOGS_DIR}/eval_he_${TIMESTAMP}_base.log"
BASE_LOG_FILE_RU="${LOGS_DIR}/eval_ru_${TIMESTAMP}_base.log"
BASE_LOG_FILE_EN="${LOGS_DIR}/eval_en_${TIMESTAMP}_base.log"
BASE_REPORT_FILE_HE="${LOGS_DIR}/eval_he_${TIMESTAMP}_base.flat.report"
BASE_REPORT_FILE_RU="${LOGS_DIR}/eval_ru_${TIMESTAMP}_base.flat.report"
BASE_REPORT_FILE_EN="${LOGS_DIR}/eval_en_${TIMESTAMP}_base.flat.report"

EXP_LOG_FILE_HE="${LOGS_DIR}/eval_he_${TIMESTAMP}_exp.log"
EXP_LOG_FILE_RU="${LOGS_DIR}/eval_ru_${TIMESTAMP}_exp.log"
EXP_LOG_FILE_EN="${LOGS_DIR}/eval_en_${TIMESTAMP}_exp.log"
EXP_REPORT_FILE_HE="${LOGS_DIR}/eval_he_${TIMESTAMP}_exp.flat.report"
EXP_REPORT_FILE_RU="${LOGS_DIR}/eval_ru_${TIMESTAMP}_exp.flat.report"
EXP_REPORT_FILE_EN="${LOGS_DIR}/eval_en_${TIMESTAMP}_exp.flat.report"

LOG_FILE_HTML="${LOGS_DIR}/eval_html_${TIMESTAMP}.log"
HTML_FILE="${LOGS_DIR}/eval_report_${TIMESTAMP}.html"

cd ${BASE_DIR}
./archive-backend eval --server=$1 --eval_set=${RECALL_SET_DIR}/he.recall.csv --flat_report=${BASE_REPORT_FILE_HE} >> ${BASE_LOG_FILE_HE} 2>&1 &
./archive-backend eval --server=$2 --eval_set=${RECALL_SET_DIR}/he.recall.csv --flat_report=${EXP_REPORT_FILE_HE} >> ${EXP_LOG_FILE_HE} 2>&1 &
wait

./archive-backend eval --server=$1 --eval_set=${RECALL_SET_DIR}/ru.recall.csv --flat_report=${BASE_REPORT_FILE_RU} >> ${BASE_LOG_FILE_RU} 2>&1 &
./archive-backend eval --server=$2 --eval_set=${RECALL_SET_DIR}/ru.recall.csv --flat_report=${EXP_REPORT_FILE_RU} >> ${EXP_LOG_FILE_RU} 2>&1 &
wait

./archive-backend eval --server=$1 --eval_set=${RECALL_SET_DIR}/en.recall.csv --flat_report=${BASE_REPORT_FILE_EN} >> ${BASE_LOG_FILE_EN} 2>&1 &
./archive-backend eval --server=$2 --eval_set=${RECALL_SET_DIR}/en.recall.csv --flat_report=${EXP_REPORT_FILE_EN} >> ${EXP_LOG_FILE_EN} 2>&1 &
wait

./archive-backend vs_golden_html --flat_reports=${EXP_REPORT_FILE_HE},${EXP_REPORT_FILE_EN},${EXP_REPORT_FILE_RU} --golden_flat_reports=${BASE_REPORT_FILE_HE},${BASE_REPORT_FILE_RU},${BASE_REPORT_FILE_EN} --vs_golden_html=${HTML_FILE} >> ${LOG_FILE_HTML} 2>&1

# Cleanup old logs (older then week).
find ${LOGS_DIR} -name "eval_he_*.log" -type f -mtime +7 -exec rm -f {} \;
find ${LOGS_DIR} -name "eval_ru_*.log" -type f -mtime +7 -exec rm -f {} \;
find ${LOGS_DIR} -name "eval_en_*.log" -type f -mtime +7 -exec rm -f {} \;
find ${LOGS_DIR} -name "eval_report_*.log" -type f -mtime +7 -exec rm -f {} \;

echo "Done writing to: ${HTML_FILE}"

exit 0
