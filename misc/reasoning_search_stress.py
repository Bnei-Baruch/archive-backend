#!/usr/bin/env python
# -*- coding: utf-8 -*-
from __future__ import print_function, unicode_literals

"""
Stress test the reasoning search background flow.

Works with Python 2.7+ and Python 3 without external packages.

Example:
  ./misc/reasoning_search_stress.py \
    --base-url http://172.18.0.7:8080 \
    --requests 30 \
    --concurrency 20 \
    --mode mixed

Outputs:
  - runs.jsonl: one JSON object per search request
  - runs.csv: compact per-search metrics
  - resources.csv: host, backend, Elasticsearch, and Postgres samples
  - summary.json: aggregate search result and peak resource usage
"""

import argparse
import codecs
import json
import os
import subprocess
import sys
import threading
import time
from datetime import datetime

PY2 = sys.version_info[0] == 2
if PY2:
    import Queue as queue
    import urllib as urllib_parse
    import urllib2 as urllib_request
    text_type = unicode  # noqa: F821
    binary_type = str
else:
    import queue
    import urllib.parse as urllib_parse
    import urllib.request as urllib_request
    import urllib.error as urllib_error
    text_type = str
    binary_type = bytes


DEFAULT_QUERIES = [
    "The Last Generation meaning",
    "four phases of direct light",
    "where Rav talks about Moses as the point in the heart",
    "taste of life 2024",
    "love of friends",
    "weekly Torah portion from last Saturday",
    "Baal HaSulam The Peace lesson from 2023",
    "where Rav talks about Rabash importance of society",
    "point in the heart in a morning lesson from Sunday",
    "Talmud Eser Sefirot restriction lesson part 2",
    "mutual guarantee lesson from the 2022 convention",
    "Kabbalah for beginners",
    "friends gathering lesson from a congress",
    "Kabbalah convention lesson about opening the heart",
    "where Rav talks about reflected light and screen",
    "intention to bestow in a lesson from 2021",
    "ten sefirot lesson from Monday morning",
    "what is prayer in Kabbalah according to Rav",
    "where Rav talks about love your neighbor as yourself",
    "Shamati there is none else besides Him lesson from 2020",
    "purpose of life TV program from 2019",
    "love of friends lesson from the Arava convention",
    "Zohar introduction lesson part 1",
    "Baal HaSulam freedom article lesson",
    "Rabash society articles lesson from Wednesday",
    "where Rav talks about connection in the ten during congress",
    "lesson from the World Kabbalah Convention about mutual guarantee",
    "where Rav talks about the role of women in the world kli",
    "daily lesson from 2024 about faith above reason",
    "where Rav explains why Moses did not enter the land of Israel",
]

TERMINAL_STATES = set(["completed", "failed", "canceled"])

DEFAULT_DOCKER_CONTAINERS = [
    ("backend", "archive-docker-archive_backend-1"),
    ("elasticsearch", "archive-docker-elastic-1"),
    ("postgres", "archive-docker-postgres_mdb-1"),
]


class MethodRequest(urllib_request.Request):
    def __init__(self, url, data=None, headers=None, method=None):
        urllib_request.Request.__init__(self, url, data=data, headers=headers or {})
        self._method = method

    def get_method(self):
        return self._method or urllib_request.Request.get_method(self)


def to_bytes(value):
    if value is None:
        return None
    if PY2:
        if isinstance(value, text_type):
            return value.encode("utf-8")
        return value
    if isinstance(value, binary_type):
        return value
    return value.encode("utf-8")


def to_text(value):
    if value is None:
        return ""
    if isinstance(value, text_type):
        return value
    if isinstance(value, binary_type):
        return value.decode("utf-8", "replace")
    return text_type(value)


def json_dumps(value, pretty=False):
    kwargs = {"ensure_ascii": False}
    if pretty:
        kwargs["indent"] = 2
    return json.dumps(value, **kwargs)


def compact_text(value, limit=500):
    text = to_text(value).replace("\n", " ").replace("\r", " ").strip()
    if len(text) > limit:
        return text[:limit] + "..."
    return text


def payload_preview(payload):
    if payload is None:
        return ""
    return compact_text(json_dumps(payload), 700)


def print_line(value):
    text = to_text(value)
    if PY2:
        sys.stdout.write(text.encode("utf-8") + "\n")
    else:
        sys.stdout.write(text + "\n")
    sys.stdout.flush()


def open_text(path, mode):
    return codecs.open(path, mode, encoding="utf-8")


def csv_escape(value):
    text = to_text(value)
    text = text.replace('"', '""')
    if ',' in text or '"' in text or '\n' in text or '\r' in text:
        return '"' + text + '"'
    return text


def write_csv_row(handle, fields, row):
    handle.write(",".join([csv_escape(row.get(field, "")) for field in fields]) + "\n")


def parse_args():
    parser = argparse.ArgumentParser(description="Stress test reasoning search.")
    parser.add_argument("--base-url", required=True, help="Backend base URL, e.g. http://127.0.0.1:8080")
    parser.add_argument("--requests", type=int, default=30, help="Total searches to run")
    parser.add_argument("--concurrency", type=int, default=20, help="Concurrent searches")
    parser.add_argument("--mode", choices=["regular", "rapid", "mixed"], default="rapid")
    parser.add_argument("--ui-language", default="he")
    parser.add_argument("--debug", dest="debug", action="store_true", default=True, help="Send deb=true; enabled by default to avoid cached search responses")
    parser.add_argument("--no-debug", dest="debug", action="store_false", help="Send deb=false and allow normal cache behavior")
    parser.add_argument("--queries-file", help="One query per line. Blank lines and # comments are ignored")
    parser.add_argument("--poll-interval", type=float, default=2.0)
    parser.add_argument("--timeout", type=float, default=420.0, help="Per-search timeout in seconds")
    parser.add_argument("--resource-interval", type=float, default=2.0)
    parser.add_argument(
        "--docker-container",
        action="append",
        dest="docker_containers",
        default=[],
        help="Container name/id to sample with docker stats. Repeat for multiple containers; defaults to backend, Elasticsearch, and Postgres.",
    )
    parser.add_argument("--no-docker-stats", action="store_true", help="Disable Docker resource sampling")
    parser.add_argument("--output-dir", help="Defaults to stress-results/<timestamp>")
    parser.add_argument(
        "--header",
        action="append",
        default=[],
        help="Extra HTTP header, e.g. 'Authorization: Bearer TOKEN'. Can be repeated.",
    )
    args = parser.parse_args()
    if args.no_docker_stats:
        args.docker_containers = []
    elif not args.docker_containers:
        args.docker_containers = [item[1] for item in DEFAULT_DOCKER_CONTAINERS]
    return args


def load_queries(path):
    if not path:
        return DEFAULT_QUERIES
    queries = []
    with open_text(path, "r") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#"):
                queries.append(line)
    if not queries:
        raise ValueError("queries file did not contain any queries")
    return queries


def request_json(method, url, body=None, headers=None, timeout=30):
    payload = None
    req_headers = {"Accept": "application/json"}
    if body is not None:
        payload = to_bytes(json_dumps(body))
        req_headers["Content-Type"] = "application/json"
    for header in headers or []:
        if ":" not in header:
            raise ValueError("invalid header %r; expected 'Name: value'" % header)
        name, value = header.split(":", 1)
        req_headers[name.strip()] = value.strip()

    request_url = to_bytes(url) if PY2 else url
    req = MethodRequest(request_url, data=payload, headers=req_headers, method=method)
    try:
        resp = urllib_request.urlopen(req, timeout=timeout)
        try:
            text = to_text(resp.read())
            status = resp.getcode()
            if not text:
                return status, {}
            try:
                return status, json.loads(text)
            except ValueError:
                return status, {"raw_response": text}
        finally:
            resp.close()
    except Exception as e:
        http_error = getattr(urllib_request, "HTTPError", None)
        if not PY2:
            http_error = urllib_error.HTTPError
        if http_error is not None and isinstance(e, http_error):
            text = to_text(e.read())
            try:
                parsed = json.loads(text)
            except ValueError:
                parsed = {"raw_error": text}
            return e.code, parsed
        raise


def endpoint(base_url, path, params=None):
    url = base_url.rstrip("/") + path
    if params:
        url += "?" + urllib_parse.urlencode(params)
    return url


def payload_error_reason(payload):
    if isinstance(payload, dict):
        error = payload.get("error")
        if isinstance(error, dict):
            return to_text(error.get("message") or json_dumps(error))
        if error:
            return to_text(error)
        for key in ("message", "title", "detail", "raw_error", "raw_response"):
            if payload.get(key):
                return to_text(payload[key])
    return payload_preview(payload)


def run_search(index, query, is_rapid, args):
    started = time.time()
    result = {
        "index": index,
        "query": query,
        "is_rapid": is_rapid,
        "started_at": datetime.utcnow().isoformat() + "Z",
        "session_id": "",
        "state": "not_started",
        "phase": "",
        "polls": 0,
        "status_http": 0,
        "start_latency_ms": 0,
        "total_latency_ms": 0,
        "rapid_first_results_ms": None,
        "result_count": 0,
        "no_results": None,
        "cache_hit": None,
        "used_tokens": None,
        "error_reason": "",
        "error_payload": None,
        "error": "",
        "error_preview": "",
    }

    body = {
        "q": query,
        "ui_language": args.ui_language,
        "is_rapid": is_rapid,
        "deb": args.debug,
    }

    current_request = ""
    try:
        start_begin = time.time()
        start_url = endpoint(args.base_url, "/search/reasoning/start")
        current_request = "POST %s" % start_url
        status_code, payload = request_json(
            "POST",
            start_url,
            body=body,
            headers=args.header,
            timeout=30,
        )
        result["start_latency_ms"] = int((time.time() - start_begin) * 1000)
        result["status_http"] = status_code
        if status_code >= 300:
            result["state"] = "start_failed"
            result["error_reason"] = payload_error_reason(payload)
            result["error_payload"] = payload
            result["error"] = "POST %s returned HTTP %s: %s" % (start_url, status_code, payload_preview(payload))
            result["error_preview"] = compact_text(result["error"], 300)
            return result

        session_id = payload.get("session_id", "")
        result["session_id"] = session_id
        if not session_id:
            result["state"] = "start_failed"
            result["error_reason"] = "response has no session_id"
            result["error_payload"] = payload
            result["error"] = "POST %s returned HTTP %s but response has no session_id: %s" % (
                start_url,
                status_code,
                payload_preview(payload),
            )
            result["error_preview"] = compact_text(result["error"], 300)
            return result

        deadline = started + args.timeout
        while time.time() < deadline:
            time.sleep(args.poll_interval)
            result["polls"] += 1
            status_url = endpoint(args.base_url, "/search/reasoning/status", {"session_id": session_id})
            current_request = "GET %s" % status_url
            status_code, status = request_json(
                "GET",
                status_url,
                headers=args.header,
                timeout=20,
            )
            result["status_http"] = status_code
            if status_code >= 300:
                result["state"] = "status_failed"
                result["error_reason"] = payload_error_reason(status)
                result["error_payload"] = status
                result["error"] = "GET %s returned HTTP %s: %s" % (status_url, status_code, payload_preview(status))
                result["error_preview"] = compact_text(result["error"], 300)
                return result

            state = status.get("state", "")
            result["state"] = state
            result["phase"] = status.get("phase", "")

            if (
                result["rapid_first_results_ms"] is None
                and status.get("rapid_results_available")
                and status.get("rapid_results")
            ):
                result["rapid_first_results_ms"] = int((time.time() - started) * 1000)

            if state in TERMINAL_STATES:
                if state != "completed":
                    result["error_reason"] = status.get("error") or status.get("message") or state
                    result["error_payload"] = status
                    result["error"] = result["error_reason"]
                    result["error_preview"] = compact_text(result["error_reason"], 300)
                break

        if result["state"] not in TERMINAL_STATES:
            result["state"] = "timeout"
            result["error_reason"] = "timed out after %ss" % args.timeout
            result["error"] = "timed out after %ss" % args.timeout
            result["error_preview"] = result["error"]
            return result

        if result["state"] == "completed":
            result_url = endpoint(args.base_url, "/search/reasoning/result", {"session_id": session_id})
            current_request = "GET %s" % result_url
            status_code, final_payload = request_json(
                "GET",
                result_url,
                headers=args.header,
                timeout=60,
            )
            result["status_http"] = status_code
            if status_code >= 300:
                result["state"] = "result_failed"
                result["error_reason"] = payload_error_reason(final_payload)
                result["error_payload"] = final_payload
                result["error"] = "GET %s returned HTTP %s: %s" % (result_url, status_code, payload_preview(final_payload))
                result["error_preview"] = compact_text(result["error"], 300)
                return result
            result["result_count"] = len(final_payload.get("results") or [])
            result["no_results"] = final_payload.get("no_results")
            result["cache_hit"] = final_payload.get("cache_hit")
            result["used_tokens"] = final_payload.get("used_tokens")

        return result
    except Exception as e:
        result["state"] = "exception"
        result["error_reason"] = repr(e)
        if current_request:
            result["error"] = "%s failed: %s" % (current_request, repr(e))
        else:
            result["error"] = repr(e)
        result["error_preview"] = compact_text(result["error"], 300)
        return result
    finally:
        result["total_latency_ms"] = int((time.time() - started) * 1000)


def read_host_stats():
    stats = {
        "cpu_count": "",
        "load1": "",
        "load5": "",
        "load15": "",
        "mem_total_kb": "",
        "mem_available_kb": "",
    }
    try:
        stats["cpu_count"] = os.sysconf(os.sysconf_names["SC_NPROCESSORS_ONLN"])
    except (KeyError, TypeError, ValueError, OSError, AttributeError):
        pass
    try:
        with open("/proc/loadavg", "r") as f:
            parts = f.read().split()
            stats["load1"], stats["load5"], stats["load15"] = parts[:3]
    except EnvironmentError:
        pass

    try:
        with open("/proc/meminfo", "r") as f:
            meminfo = {}
            for line in f:
                key, value = line.split(":", 1)
                meminfo[key] = value.strip().split()[0]
            stats["mem_total_kb"] = meminfo.get("MemTotal", "")
            stats["mem_available_kb"] = meminfo.get("MemAvailable", "")
    except EnvironmentError:
        pass
    return stats


def read_docker_stats(containers):
    if not containers:
        return {}
    try:
        process = subprocess.Popen(
            ["docker", "stats", "--no-stream", "--format", "{{json .}}"] + containers,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        stdout, stderr = process.communicate()
    except EnvironmentError as e:
        return dict((container, {"docker_error": repr(e)}) for container in containers)
    lines = to_text(stdout).strip().splitlines()
    parsed = []
    for line in lines:
        try:
            item = json.loads(line)
        except ValueError:
            continue
        parsed.append(item)
    error = to_text(stderr).strip()
    if process.returncode != 0 and not error:
        error = "docker stats failed"
    stats = {}
    for container in containers:
        for item in parsed:
            if item.get("Name") == container or to_text(item.get("ID", "")).startswith(container):
                stats[container] = item
                break
        if container not in stats:
            stats[container] = {"docker_error": error or "empty docker stats output"}
    return stats


def container_role(container):
    for role, default_container in DEFAULT_DOCKER_CONTAINERS:
        if container == default_container:
            return role
    return container


def number(value):
    try:
        return float(to_text(value).strip().rstrip("%"))
    except (TypeError, ValueError):
        return None


def build_resource_row(host, container, role, docker):
    return {
        "ts": datetime.utcnow().isoformat() + "Z",
        "cpu_count": host.get("cpu_count", ""),
        "load1": host.get("load1", ""),
        "load5": host.get("load5", ""),
        "load15": host.get("load15", ""),
        "mem_total_kb": host.get("mem_total_kb", ""),
        "mem_available_kb": host.get("mem_available_kb", ""),
        "container_role": role,
        "container": docker.get("Name", container),
        "container_cpu_perc": docker.get("CPUPerc", ""),
        "container_mem_usage": docker.get("MemUsage", ""),
        "container_mem_perc": docker.get("MemPerc", ""),
        "container_net_io": docker.get("NetIO", ""),
        "container_block_io": docker.get("BlockIO", ""),
        "container_pids": docker.get("PIDs", ""),
        "docker_error": docker.get("docker_error", ""),
    }


def update_resource_summary(summary, host, role, container, docker):
    host_summary = summary["host"]
    cpu_count = number(host.get("cpu_count"))
    load1 = number(host.get("load1"))
    available_kb = number(host.get("mem_available_kb"))
    total_kb = number(host.get("mem_total_kb"))
    if cpu_count:
        host_summary["cpu_count"] = int(cpu_count)
    if load1 is not None:
        host_summary["max_load1"] = max(host_summary.get("max_load1", 0), load1)
        if cpu_count:
            host_summary["max_load1_per_cpu"] = max(host_summary.get("max_load1_per_cpu", 0), load1 / cpu_count)
    if available_kb is not None:
        current_min = host_summary.get("min_mem_available_kb")
        host_summary["min_mem_available_kb"] = available_kb if current_min is None else min(current_min, available_kb)
        if total_kb:
            used_percent = 100.0 * (total_kb - available_kb) / total_kb
            host_summary["max_mem_used_percent"] = max(host_summary.get("max_mem_used_percent", 0), used_percent)

    if not container:
        return
    container_summary = summary["containers"].setdefault(role, {
        "container": container,
        "max_cpu_percent": 0,
        "max_mem_percent": 0,
        "max_pids": 0,
        "samples": 0,
        "sample_errors": 0,
    })
    container_summary["samples"] += 1
    cpu_percent = number(docker.get("CPUPerc"))
    mem_percent = number(docker.get("MemPerc"))
    pids = number(docker.get("PIDs"))
    if cpu_percent is not None:
        container_summary["max_cpu_percent"] = max(container_summary["max_cpu_percent"], cpu_percent)
    if mem_percent is not None:
        container_summary["max_mem_percent"] = max(container_summary["max_mem_percent"], mem_percent)
    if pids is not None:
        container_summary["max_pids"] = max(container_summary["max_pids"], int(pids))
    if docker.get("docker_error"):
        container_summary["sample_errors"] += 1


def resource_sampler(args, stop_event, output_path, resource_summary):
    fields = [
        "ts",
        "cpu_count",
        "load1",
        "load5",
        "load15",
        "mem_total_kb",
        "mem_available_kb",
        "container_role",
        "container",
        "container_cpu_perc",
        "container_mem_usage",
        "container_mem_perc",
        "container_net_io",
        "container_block_io",
        "container_pids",
        "docker_error",
    ]
    with open_text(output_path, "w") as f:
        write_csv_row(f, fields, dict((field, field) for field in fields))
        f.flush()
        while not stop_event.is_set():
            sample_started = time.time()
            host = read_host_stats()
            docker_stats = read_docker_stats(args.docker_containers)
            if args.docker_containers:
                for container in args.docker_containers:
                    role = container_role(container)
                    docker = docker_stats.get(container, {})
                    write_csv_row(f, fields, build_resource_row(host, container, role, docker))
                    update_resource_summary(resource_summary, host, role, container, docker)
            else:
                write_csv_row(f, fields, build_resource_row(host, "", "host", {}))
                update_resource_summary(resource_summary, host, "host", "", {})
            f.flush()
            stop_event.wait(max(0, args.resource_interval - (time.time() - sample_started)))


def writer_thread(output_dir, results_queue, stop_event):
    jsonl_path = os.path.join(output_dir, "runs.jsonl")
    csv_path = os.path.join(output_dir, "runs.csv")
    csv_fields = [
        "index",
        "query",
        "is_rapid",
        "session_id",
        "state",
        "phase",
        "polls",
        "status_http",
        "start_latency_ms",
        "total_latency_ms",
        "rapid_first_results_ms",
        "result_count",
        "no_results",
        "cache_hit",
        "used_tokens",
        "error_reason",
        "error_payload",
        "error_preview",
        "error",
    ]
    with open_text(jsonl_path, "w") as jsonl, open_text(csv_path, "w") as csvfile:
        write_csv_row(csvfile, csv_fields, dict((field, field) for field in csv_fields))
        while not stop_event.is_set() or not results_queue.empty():
            try:
                result = results_queue.get(timeout=0.2)
            except queue.Empty:
                continue
            jsonl.write(json_dumps(result) + "\n")
            jsonl.flush()
            csv_result = dict(result)
            if csv_result.get("error_payload") is not None:
                csv_result["error_payload"] = json_dumps(csv_result["error_payload"])
            write_csv_row(csvfile, csv_fields, csv_result)
            csvfile.flush()
            line = (
                "[%03d] %-13s %7.1fs results=%-2s rapid=%-5s query=%s"
                % (
                    result["index"],
                    result["state"],
                    result["total_latency_ms"] / 1000.0,
                    result.get("result_count", 0),
                    text_type(result["is_rapid"]).lower(),
                    result["query"],
                )
            )
            if result.get("error_preview"):
                line += " error=%s" % result["error_preview"]
            print_line(line)


def run_all(planned, args, results_queue):
    work_queue = queue.Queue()
    results = []
    results_lock = threading.Lock()

    def worker():
        while True:
            item = work_queue.get()
            try:
                if item is None:
                    return
                index, query, is_rapid = item
                result = run_search(index, query, is_rapid, args)
                with results_lock:
                    results.append(result)
                results_queue.put(result)
            finally:
                work_queue.task_done()

    worker_count = min(args.concurrency, len(planned))
    threads = []
    for _ in range(worker_count):
        thread = threading.Thread(target=worker)
        thread.daemon = True
        thread.start()
        threads.append(thread)

    for item in planned:
        work_queue.put(item)
    for _ in threads:
        work_queue.put(None)

    work_queue.join()
    for thread in threads:
        thread.join(1)
    return results


def summarize(results, output_dir, args, resource_summary):
    latencies = sorted([r["total_latency_ms"] for r in results])
    completed = [r for r in results if r["state"] == "completed"]
    failed = [r for r in results if r["state"] != "completed"]

    def percentile(values, pct):
        if not values:
            return None
        index = min(len(values) - 1, int(round((pct / 100.0) * (len(values) - 1))))
        return values[index]

    summary = {
        "base_url": args.base_url,
        "requests": len(results),
        "concurrency": args.concurrency,
        "mode": args.mode,
        "completed": len(completed),
        "failed": len(failed),
        "p50_latency_ms": percentile(latencies, 50),
        "p95_latency_ms": percentile(latencies, 95),
        "max_latency_ms": max(latencies) if latencies else None,
        "total_used_tokens": sum([(r.get("used_tokens") or 0) for r in completed]),
        "resources": resource_summary,
        "state_counts": {},
        "failed_examples": [],
        "output_dir": output_dir,
        "output_files": {
            "runs_jsonl": os.path.join(output_dir, "runs.jsonl"),
            "runs_csv": os.path.join(output_dir, "runs.csv"),
            "resources_csv": os.path.join(output_dir, "resources.csv"),
            "summary_json": os.path.join(output_dir, "summary.json"),
        },
    }
    for result in results:
        summary["state_counts"][result["state"]] = summary["state_counts"].get(result["state"], 0) + 1
        if result["state"] != "completed" and len(summary["failed_examples"]) < 5:
            summary["failed_examples"].append({
                "index": result.get("index"),
                "state": result.get("state"),
                "status_http": result.get("status_http"),
                "query": result.get("query"),
                "error_reason": result.get("error_reason") or result.get("error"),
                "error_payload": result.get("error_payload"),
            })

    path = os.path.join(output_dir, "summary.json")
    with open_text(path, "w") as f:
        f.write(json_dumps(summary, pretty=True) + "\n")
    return summary


def file_size(path):
    try:
        return os.path.getsize(path)
    except EnvironmentError:
        return 0


def print_output_files(output_dir):
    files = [
        ("runs_jsonl", os.path.join(output_dir, "runs.jsonl")),
        ("runs_csv", os.path.join(output_dir, "runs.csv")),
        ("resources_csv", os.path.join(output_dir, "resources.csv")),
        ("summary_json", os.path.join(output_dir, "summary.json")),
    ]
    print_line("Output files:")
    for name, path in files:
        print_line("  %s: %s (%d bytes)" % (name, path, file_size(path)))


def main():
    args = parse_args()
    queries = load_queries(args.queries_file)
    timestamp = datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")
    output_dir = args.output_dir or os.path.join("stress-results", timestamp)
    if not os.path.isdir(output_dir):
        os.makedirs(output_dir)

    planned = []
    for i in range(args.requests):
        if args.mode == "mixed":
            is_rapid = i % 2 == 0
        else:
            is_rapid = args.mode == "rapid"
        planned.append((i + 1, queries[i % len(queries)], is_rapid))

    stop_resources = threading.Event()
    resource_summary = {"host": {}, "containers": {}}
    resource_thread = threading.Thread(
        target=resource_sampler,
        args=(args, stop_resources, os.path.join(output_dir, "resources.csv"), resource_summary),
    )
    resource_thread.daemon = True
    resource_thread.start()

    results_queue = queue.Queue()
    stop_writer = threading.Event()
    writer = threading.Thread(target=writer_thread, args=(output_dir, results_queue, stop_writer))
    writer.daemon = True
    writer.start()

    print_line(
        "Starting %d searches with concurrency=%d, mode=%s, deb=%s"
        % (args.requests, args.concurrency, args.mode, text_type(args.debug).lower())
    )
    print_line("Writing results to %s" % output_dir)
    if args.docker_containers:
        print_line("Sampling Docker containers: %s" % ", ".join(args.docker_containers))
    else:
        print_line("Docker resource sampling disabled")

    started = time.time()
    try:
        results = run_all(planned, args, results_queue)
    finally:
        stop_writer.set()
        writer.join(5)
        stop_resources.set()
        resource_thread.join(5)

    summary = summarize(results, output_dir, args, resource_summary)
    elapsed = time.time() - started
    print_line(json_dumps(summary, pretty=True))
    print_output_files(output_dir)
    print_line("Elapsed: %.1fs" % elapsed)

    return 0 if summary["failed"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
