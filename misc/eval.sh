#!/usr/bin/env bash

set +e
set -x

export LANG=en_US.UTF-8

BASE_DIR="/sites/archive-backend"
RECALL_SET_DIR="${BASE_DIR}/search/data"
LOGS_DIR="${BASE_DIR}/logs/es"
GOLDEN_DIR="${BASE_DIR}/search/golden"
BACKEND="https://kabbalahmedia.info/backend"

# LOCAL
#BASE_DIR="."
#RECALL_SET_DIR="${BASE_DIR}/search/data"
#LOGS_DIR="/tmp/logs/es"
#GOLDEN_DIR="/tmp/search-golden"
#BACKEND="https://kabbalahmedia.info/backend"

TIMESTAMP="$(date '+%Y%m%d%H%M%S')"

LOG_FILE_HE="${LOGS_DIR}/eval_he_${TIMESTAMP}.log"
LOG_FILE_RU="${LOGS_DIR}/eval_ru_${TIMESTAMP}.log"
LOG_FILE_EN="${LOGS_DIR}/eval_en_${TIMESTAMP}.log"
LOG_FILE_HTML="${LOGS_DIR}/eval_html_${TIMESTAMP}.log"
REPORT_FILE_HE="${LOGS_DIR}/eval_he_${TIMESTAMP}.flat.report"
REPORT_FILE_RU="${LOGS_DIR}/eval_ru_${TIMESTAMP}.flat.report"
REPORT_FILE_EN="${LOGS_DIR}/eval_en_${TIMESTAMP}.flat.report"
HTML_FILE="${LOGS_DIR}/eval_report_${TIMESTAMP}.html"
GOLDEN_REPORT_FILE_HE="$(ls ${GOLDEN_DIR}/eval_he_*.flat.report)"
GOLDEN_REPORT_FILE_RU="$(ls ${GOLDEN_DIR}/eval_ru_*.flat.report)"
GOLDEN_REPORT_FILE_EN="$(ls ${GOLDEN_DIR}/eval_en_*.flat.report)"

# Latencies logs and output
LOG_FILE_LATENCIES_CSV="${LOGS_DIR}/latency_csv_${TIMESTAMP}.log"
LOG_FILE_LATENCIES_HTML="${LOGS_DIR}/latency_html_${TIMESTAMP}.log"
LATENCIES_CSV="${LOGS_DIR}/latency_csv_${TIMESTAMP}.csv"
LATENCIES_HTML="${LOGS_DIR}/latency_html_${TIMESTAMP}.html"

cd ${BASE_DIR}
./archive-backend eval --server=${BACKEND} --eval_set=${RECALL_SET_DIR}/he.recall.csv --flat_report=${REPORT_FILE_HE} >> ${LOG_FILE_HE} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_HE} kolmanv@gmail.com
fi
./archive-backend eval --server=${BACKEND} --eval_set=${RECALL_SET_DIR}/ru.recall.csv --flat_report=${REPORT_FILE_RU} >> ${LOG_FILE_RU} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_RU} kolmanv@gmail.com
fi
./archive-backend eval --server=${BACKEND} --eval_set=${RECALL_SET_DIR}/en.recall.csv --flat_report=${REPORT_FILE_EN} >> ${LOG_FILE_EN} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_EN} kolmanv@gmail.com
fi

# Run latencies.
./archive-backend log latency --output_file=${LATENCIES_CSV} >> ${LOG_FILE_LATENCIES_CSV} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_EN} kolmanv@gmail.com
fi
./archive-backend log latency_aggregate --csv_file=${LATENCIES_CSV} --output_html=${LATENCIES_HTML} >> ${LOG_FILE_LATENCIES_HTML} 2>&1
if [ $? -ne 0 ]; then                                                                                                                                                                                                                                                                 mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_EN} kolmanv@gmail.com
fi


./archive-backend vs_golden_html --flat_reports=${REPORT_FILE_HE},${REPORT_FILE_EN},${REPORT_FILE_RU} --golden_flat_reports=${GOLDEN_REPORT_FILE_HE},${GOLDEN_REPORT_FILE_RU},${GOLDEN_REPORT_FILE_EN} --vs_golden_html=${HTML_FILE} --html_to_inject=${LATENCIES_HTML} >> ${LOG_FILE_HTML} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_HTML} kolmanv@gmail.com
fi

# Cleanup old logs and reports (older then week).
find ${LOGS_DIR} -name "*" -type f -mtime +7 -exec rm -f {} \;

mailx -s "$(echo -e "Daily Eval: Done.\nContent-Type: text/html")" \
        kolmanv@gmail.com edoshor@gmail.com eranminuchin@gmail.com yurihechter@gmail.com alex.mizrachi@gmail.com < ${HTML_FILE}

exit 0