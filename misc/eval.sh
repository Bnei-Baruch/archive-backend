#!/usr/bin/env bash

set +e
set -x

BASE_DIR="/sites/archive-backend"
TIMESTAMP="$(date '+%Y%m%d%H%M%S')"
LOG_FILE_HE="$BASE_DIR/logs/es/eval_he_$TIMESTAMP.log"
LOG_FILE_RU="$BASE_DIR/logs/es/eval_ru_$TIMESTAMP.log"
LOG_FILE_EN="$BASE_DIR/logs/es/eval_en_$TIMESTAMP.log"
REPORT_FILE_HE="$BASE_DIR/logs/es/eval_he_$TIMESTAMP.flat.report"
REPORT_FILE_RU="$BASE_DIR/logs/es/eval_ru_$TIMESTAMP.flat.report"
REPORT_FILE_EN="$BASE_DIR/logs/es/eval_en_$TIMESTAMP.flat.report"

cd ${BASE_DIR}
./archive-backend eval --server=https://kabbalahmedia.info/backend --eval_set=./search/data/he.recall.csv --flat_report=${REPORT_FILE_HE} >> ${LOG_FILE_HE} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_HE} kolmanv@gmail.com
fi
./archive-backend eval --server=https://kabbalahmedia.info/backend --eval_set=./search/data/ru.recall.csv --flat_report=${REPORT_FILE_RU} >> ${LOG_FILE_RU} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_RU} kolmanv@gmail.com
fi
./archive-backend eval --server=https://kabbalahmedia.info/backend --eval_set=./search/data/en.recall.csv --flat_report=${REPORT_FILE_EN} >> ${LOG_FILE_EN} 2>&1
if [ $? -ne 0 ]; then
    mail -s "Daily Eval: Error." -r "mdb@bbdomain.org" -a ${LOG_FILE_EN} kolmanv@gmail.com
fi

RECALL_HE="$(cat ${LOG_FILE_HE} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"
RECALL_RU="$(cat ${LOG_FILE_RU} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"
RECALL_EN="$(cat ${LOG_FILE_EN} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"

# Cleanup old logs (older then week).
find ${BASE_DIR}/logs/es -name "eval_he_*.log" -type f -mtime +7 -exec rm -f {} \;
find ${BASE_DIR}/logs/es -name "eval_ru_*.log" -type f -mtime +7 -exec rm -f {} \;
find ${BASE_DIR}/logs/es -name "eval_en_*.log" -type f -mtime +7 -exec rm -f {} \;

echo "Done.\nHE\n${RECALL_HE}\nRU\n${RECALL_RU}\nEN\n${RECALL_EN}" | mail -s "Daily Eval: Done." -r "mdb@bbdomain.org" -a ${REPORT_FILE_HE} -a ${REPORT_FILE_RU} \
    -a ${REPORT_FILE_EN} -a ${LOG_FILE_HE} -a ${LOG_FILE_RU} -a ${LOG_FILE_EN} \
    kolmanv@gmail.com edoshor@gmail.com kolmanv@gmail.com eranminuchin@gmail.com yurihechter@gmail.com
exit 0
