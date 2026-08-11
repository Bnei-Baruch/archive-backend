#!/usr/bin/env bash

# Capture one short production baseline during normal peak traffic.
# This script only observes production; it does not send search requests.

set -u

DURATION="${DURATION:-600}"
INTERVAL=2
DOCKER_STATS_TIMEOUT="${DOCKER_STATS_TIMEOUT:-10}"
LATENCY_BUCKET_MS="${LATENCY_BUCKET_MS:-100}"
MAX_LATENCY_SECONDS="${MAX_LATENCY_SECONDS:-600}"
ACCESS_LOG="${ACCESS_LOG:-/sites/archive-backend/logs/nginx-access.log}"
OUTPUT_DIR="${OUTPUT_DIR:-$HOME/prod-peak-$(date -u +%Y%m%dT%H%M%SZ)}"

BACKEND_CONTAINER="archive-docker-archive_backend-1"
ELASTICSEARCH_CONTAINER="archive-docker-elastic-1"
POSTGRES_CONTAINER="archive-docker-postgres_mdb-1"

if ! [[ "$DURATION" =~ ^[1-9][0-9]*$ ]]; then
  echo "DURATION must be a positive number of seconds" >&2
  exit 1
fi

if ! [[ "$DOCKER_STATS_TIMEOUT" =~ ^[1-9][0-9]*$ ]] || \
   ! [[ "$LATENCY_BUCKET_MS" =~ ^[1-9][0-9]*$ ]] || \
   ! [[ "$MAX_LATENCY_SECONDS" =~ ^[1-9][0-9]*$ ]]; then
  echo "Docker timeout and latency histogram settings must be positive numbers" >&2
  exit 1
fi

if [ ! -r "$ACCESS_LOG" ]; then
  echo "Cannot read $ACCESS_LOG. Run with sufficient permissions or set ACCESS_LOG." >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

ACCESS_PID=""
VMSTAT_PID=""
DOCKER_PID=""

cleanup() {
  [ -z "$ACCESS_PID" ] || kill "$ACCESS_PID" 2>/dev/null || true
  [ -z "$VMSTAT_PID" ] || kill "$VMSTAT_PID" 2>/dev/null || true
  [ -z "$DOCKER_PID" ] || kill "$DOCKER_PID" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 130' INT TERM

{
  echo "started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "duration_seconds=$DURATION"
  echo "cpu_count=$(nproc)"
  uptime
  free -h
  swapon --show 2>/dev/null || true
} > "$OUTPUT_DIR/host-start.txt"

# Parse the repository's quoted Nginx "main" format while traffic streams in.
# Fixed-size buckets avoid storing the raw access log or sorting all latencies.
timeout "${DURATION}s" tail -n 0 -F "$ACCESS_LOG" \
  | awk -v duration="$DURATION" \
      -v bucket_ms="$LATENCY_BUCKET_MS" \
      -v max_latency_seconds="$MAX_LATENCY_SECONDS" '
function percentile(fraction, target, cumulative, bucket) {
  target=requests*fraction
  if (target > int(target)) target=int(target)+1
  if (target < 1) target=1

  for (bucket=0; bucket<=max_bucket; bucket++) {
    cumulative+=histogram[bucket]
    if (cumulative >= target)
      return sprintf("%.3f", (bucket+1)*bucket_ms/1000)
  }
  return ">" max_latency_seconds
}
BEGIN {
  max_bucket=int(max_latency_seconds*1000/bucket_ms)
}
{
  # Expected tail: "request" status bytes "referer" "agent" "forwarded-for" request_time upstream_time pipe
  if (split($0, quoted, "\"") != 9) {
    format_errors++
    next
  }

  request=quoted[2]
  metadata=quoted[3]
  sub(/^[[:space:]]+/, "", metadata)
  sub(/[[:space:]]+$/, "", metadata)
  metadata_count=split(metadata, metadata_fields, /[[:space:]]+/)
  status=metadata_fields[1]
  body_bytes=metadata_fields[2]

  timing=quoted[9]
  sub(/^[[:space:]]+/, "", timing)
  sub(/[[:space:]]+$/, "", timing)
  timing_count=split(timing, timing_fields, /[[:space:]]+/)
  request_time=timing_fields[1]
  pipe=timing_fields[timing_count]

  if (request !~ /^(GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS)[[:space:]].*[[:space:]]HTTP\/[0-9.]+$/ ||
      metadata_count != 2 || status !~ /^[1-5][0-9][0-9]$/ || body_bytes !~ /^[0-9]+$/ ||
      timing_count < 3 || request_time !~ /^[0-9]+([.][0-9]+)?$/ || pipe !~ /^[.p]$/) {
    format_errors++
    next
  }

  requests++
  if (status ~ /^4/) errors4++
  if (status ~ /^5/) errors5++

  latency=request_time+0
  if (latency > max_latency) max_latency=latency
  bucket=int(latency*1000/bucket_ms)
  if (bucket > max_bucket) overflow++
  else histogram[bucket]++
}
END {
  printf "requests=%d\n", requests
  printf "format_errors=%d\n", format_errors
  printf "latency_bucket_ms=%d\n", bucket_ms
  printf "latency_overflow=%d\n", overflow
  if (!requests) exit

  printf "average_rps=%.3f\n", requests/duration
  printf "4xx=%d\n", errors4
  printf "5xx=%d\n", errors5
  printf "5xx_rate_percent=%.3f\n", 100*errors5/requests
  printf "p50_seconds=%s\n", percentile(0.50)
  printf "p95_seconds=%s\n", percentile(0.95)
  printf "p99_seconds=%s\n", percentile(0.99)
  printf "max_seconds=%.3f\n", max_latency
}' > "$OUTPUT_DIR/traffic-summary.txt" &
ACCESS_PID=$!

vmstat "$INTERVAL" $((DURATION / INTERVAL + 1)) > "$OUTPUT_DIR/vmstat.txt" &
VMSTAT_PID=$!

: > "$OUTPUT_DIR/docker-stats-errors.log"
(
  echo "timestamp,cpu_count,load1,mem_available_kb,container,cpu,mem_usage,mem_percent,pids"
  END=$(( $(date +%s) + DURATION ))
  CPU_COUNT=$(nproc)

  while [ "$(date +%s)" -lt "$END" ]; do
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    LOAD1=$(cut -d ' ' -f 1 /proc/loadavg)
    MEM_AVAILABLE_KB=$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)

    if ! timeout -k 2s "${DOCKER_STATS_TIMEOUT}s" docker stats --no-stream \
        --format "$TIMESTAMP,$CPU_COUNT,$LOAD1,$MEM_AVAILABLE_KB,{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.MemPerc}},{{.PIDs}}" \
        "$BACKEND_CONTAINER" \
        "$ELASTICSEARCH_CONTAINER" \
        "$POSTGRES_CONTAINER"; then
      echo "$TIMESTAMP docker stats failed or timed out" >> "$OUTPUT_DIR/docker-stats-errors.log"
    fi

    sleep "$INTERVAL"
  done
) > "$OUTPUT_DIR/docker-stats.csv" &
DOCKER_PID=$!

echo "Collecting production baseline for $DURATION seconds..."
echo "Writing results to $OUTPUT_DIR"

wait "$ACCESS_PID" || true
wait "$VMSTAT_PID" || true
wait "$DOCKER_PID" || true

ACCESS_PID=""
VMSTAT_PID=""
DOCKER_PID=""

{
  echo "finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  uptime
  free -h
  swapon --show 2>/dev/null || true
} > "$OUTPUT_DIR/host-end.txt"

awk -F, '
NR > 1 {
  cpu=$6
  memory=$8
  gsub(/%/, "", cpu)
  gsub(/%/, "", memory)
  if (cpu+0 > max_cpu[$5]) max_cpu[$5]=cpu+0
  if (memory+0 > max_memory[$5]) max_memory[$5]=memory+0
  if ($9+0 > max_pids[$5]) max_pids[$5]=$9+0
}
END {
  for (container in max_cpu) {
    printf "%s max_cpu_percent=%.2f max_mem_percent=%.2f max_pids=%d\n",
      container, max_cpu[container], max_memory[container], max_pids[container]
  }
}' "$OUTPUT_DIR/docker-stats.csv" | sort > "$OUTPUT_DIR/container-summary.txt"

awk -F, '
NR > 1 {
  if (!samples || $3+0 > max_load1) max_load1=$3+0
  if (!samples || $4+0 < min_available) min_available=$4+0
  cpu_count=$2+0
  samples++
}
END {
  if (!samples) {
    print "no host samples"
    exit
  }
  printf "cpu_count=%d\n", cpu_count
  printf "max_load1=%.2f\n", max_load1
  printf "max_load1_per_cpu=%.4f\n", max_load1/cpu_count
  printf "min_mem_available_kb=%.0f\n", min_available
}' "$OUTPUT_DIR/docker-stats.csv" > "$OUTPUT_DIR/host-summary.txt"

echo "docker_stats_errors=$(wc -l < "$OUTPUT_DIR/docker-stats-errors.log" | tr -d " ")" \
  >> "$OUTPUT_DIR/host-summary.txt"

PARSED_REQUESTS=$(awk -F= '$1 == "requests" {print $2}' "$OUTPUT_DIR/traffic-summary.txt")
FORMAT_ERRORS=$(awk -F= '$1 == "format_errors" {print $2}' "$OUTPUT_DIR/traffic-summary.txt")
LOG_FORMAT_VALID=true
if [ "${PARSED_REQUESTS:-0}" -eq 0 ] || \
   [ $(( ${FORMAT_ERRORS:-0} * 100 )) -gt $(( ${PARSED_REQUESTS:-0} + ${FORMAT_ERRORS:-0} )) ]; then
  LOG_FORMAT_VALID=false
fi

echo
cat "$OUTPUT_DIR/traffic-summary.txt"
cat "$OUTPUT_DIR/host-summary.txt"
cat "$OUTPUT_DIR/container-summary.txt"
echo
echo "Results: $OUTPUT_DIR"

if [ "$LOG_FORMAT_VALID" != true ]; then
  echo "Access-log format validation failed; traffic and latency metrics are not reliable." >&2
  echo "Expected the quoted Nginx 'main' format from misc/nginx.conf." >&2
  exit 1
fi
