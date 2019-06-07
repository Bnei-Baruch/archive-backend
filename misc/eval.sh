#!/usr/bin/env bash

set +e
set -x

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
GOLDEN_LOG_FILE_HE="$(ls ${GOLDEN_DIR}/eval_he_*.log)"
GOLDEN_LOG_FILE_RU="$(ls ${GOLDEN_DIR}/eval_ru_*.log)"
GOLDEN_LOG_FILE_EN="$(ls ${GOLDEN_DIR}/eval_en_*.log)"
REPORT_FILE_HE="${LOGS_DIR}/eval_he_${TIMESTAMP}.flat.report"
REPORT_FILE_RU="${LOGS_DIR}/eval_ru_${TIMESTAMP}.flat.report"
REPORT_FILE_EN="${LOGS_DIR}/eval_en_${TIMESTAMP}.flat.report"
GOLDEN_REPORT_FILE_HE="$(ls ${GOLDEN_DIR}/eval_he_*.flat.report)"
GOLDEN_REPORT_FILE_RU="$(ls ${GOLDEN_DIR}/eval_ru_*.flat.report)"
GOLDEN_REPORT_FILE_EN="$(ls ${GOLDEN_DIR}/eval_en_*.flat.report)"

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

RECALL_HE="$(cat ${LOG_FILE_HE} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"
RECALL_RU="$(cat ${LOG_FILE_RU} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"
RECALL_EN="$(cat ${LOG_FILE_EN} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"
GOLDEN_RECALL_HE="$(cat ${GOLDEN_LOG_FILE_HE} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"
GOLDEN_RECALL_RU="$(cat ${GOLDEN_LOG_FILE_RU} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"
GOLDEN_RECALL_EN="$(cat ${GOLDEN_LOG_FILE_EN} | sed -n '/Unique/,/Good/p' | sed -e 's/^.*msg="//' | sed -e 's/"//')"

if [ -z "$(diff --suppress-common-lines -y <(echo "${GOLDEN_RECALL_HE}") <(echo "${RECALL_HE}"))" ]; then
        RECALL_DIFF_HE="${RECALL_HE}"
else
        RECALL_DIFF_HE="$(paste -d"|" <(echo "${GOLDEN_RECALL_HE}") <(echo "${RECALL_HE}" | sed -e 's/.*://g') | sed -e 's/|/   ===> /g')"
fi
if [ -z "$(diff --suppress-common-lines -y <(echo "${GOLDEN_RECALL_RU}") <(echo "${RECALL_RU}"))" ]; then
        RECALL_DIFF_RU="${RECALL_RU}"
else
        RECALL_DIFF_RU="$(paste -d"|" <(echo "${GOLDEN_RECALL_RU}") <(echo "${RECALL_RU}" | sed -e 's/.*://g') | sed -e 's/|/   ===> /g')"
fi
if [ -z "$(diff --suppress-common-lines -y <(echo "${GOLDEN_RECALL_EN}") <(echo "${RECALL_EN}"))" ]; then
        RECALL_DIFF_EN="${RECALL_EN}"
else
        RECALL_DIFF_EN="$(paste -d"|" <(echo "${GOLDEN_RECALL_EN}") <(echo "${RECALL_EN}" | sed -e 's/.*://g') | sed -e 's/|/   ===> /g')"
fi

QUERIES_DIFF_HE="$(diff -W1000 --suppress-common-lines -y ${GOLDEN_REPORT_FILE_HE} ${REPORT_FILE_HE} | sed -e 's/^\(.*\)\(.*\)|.*\1\(.*\)$/\1 DIFF \2: => \3/'  | sed 's/\t/ /g' | sed 's/  */ /g')"
QUERIES_DIFF_RU="$(diff -W1000 --suppress-common-lines -y ${GOLDEN_REPORT_FILE_RU} ${REPORT_FILE_RU} | sed -e 's/^\(.*\)\(.*\)|.*\1\(.*\)$/\1 DIFF \2: => \3/'  | sed 's/\t/ /g' | sed 's/  */ /g')"
QUERIES_DIFF_EN="$(diff -W1000 --suppress-common-lines -y ${GOLDEN_REPORT_FILE_EN} ${REPORT_FILE_EN} | sed -e 's/^\(.*\)\(.*\)|.*\1\(.*\)$/\1 DIFF \2: => \3/'  | sed 's/\t/ /g' | sed 's/  */ /g')"

# Cleanup old logs (older then week).
find ${LOGS_DIR} -name "eval_he_*.log" -type f -mtime +7 -exec rm -f {} \;
find ${LOGS_DIR} -name "eval_ru_*.log" -type f -mtime +7 -exec rm -f {} \;
find ${LOGS_DIR} -name "eval_en_*.log" -type f -mtime +7 -exec rm -f {} \;

echo -e "Done.\nHE\n${RECALL_DIFF_HE}\nDIFF\n${QUERIES_DIFF_HE}\n\n\nRU\n${RECALL_DIFF_RU}\nDIFF\n${QUERIES_DIFF_RU}\n\n\nEN\n${RECALL_DIFF_EN}\nDIFF\n${QUERIES_DIFF_EN}" | \
    mail -s "Daily Eval: Done." -r "mdb@bbdomain.org" -a ${REPORT_FILE_HE} -a ${REPORT_FILE_RU} \
        -a ${REPORT_FILE_EN} -a ${LOG_FILE_HE} -a ${LOG_FILE_RU} -a ${LOG_FILE_EN} \
        kolmanv@gmail.com edoshor@gmail.com eranminuchin@gmail.com yurihechter@gmail.com alex.mizrachi@gmail.com
exit 0
