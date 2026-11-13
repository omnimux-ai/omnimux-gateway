#!/usr/bin/env python3
"""Measure unchanged Vitest invocations and prove complete, disjoint shard coverage."""
import argparse
import codecs
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
import math
import os
from pathlib import Path
import re
import selectors
import signal
import statistics
import subprocess
import sys
import time

SCHEMA = 2
MAX_SECONDS = 25 * 60
ROOT = Path(__file__).resolve().parents[2]
PASS_STATUSES = {"passed", "skipped", "todo"}
TIMINGS = "scripts/ops/frontend-ci-timings.json"
PARTITION_CONTROLS = ("scripts/ops/frontend-ci.py", TIMINGS)


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"),
                                     ensure_ascii=True).encode()).hexdigest()


def utc_now():
    return datetime.now(timezone.utc).isoformat()


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def checked(command, cwd=ROOT):
    return subprocess.check_output(command, cwd=cwd, text=True, stderr=subprocess.PIPE).strip()


def identity(source, root=ROOT):
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("source must be a full 40-character commit SHA")
    if checked(["git", "rev-parse", "HEAD"], root) != source:
        raise ValueError("checkout HEAD differs from requested source")
    if checked(["git", "status", "--porcelain", "--untracked-files=all", "--", "web"], root):
        raise ValueError("web sources differ from the committed source")
    paths = ["web/vitest.config.ts", "web/package.json"]
    locks = [p for p in ("web/bun.lock", "web/bun.lockb") if (root / p).is_file()]
    if len(locks) != 1:
        raise ValueError("expected exactly one committed Bun lockfile")
    return {"source": source, "web_tree": checked(["git", "rev-parse", "HEAD:web"], root),
            "control_sha256": {p: sha256(root / p) for p in paths + locks},
            "partition_control_sha256": {p: sha256(root / p) if (root / p).is_file() else None
                                         for p in PARTITION_CONTROLS}}


def native_assignments(inventory, total):
    """Vitest 4.1.10 BaseSequencer's SHA1-path/equal-count shard contract.

    This predicts membership for verification only. Fallback execution still
    uses the original --shard flag and the installed BaseSequencer.
    """
    ordered = sorted(inventory, key=lambda name: hashlib.sha1(("/" + name).encode()).hexdigest())
    base, remainder = divmod(len(ordered), total)
    sizes = [base + (index < remainder) for index in range(total)]
    groups, offset = [], 0
    for size in sizes:
        groups.append(sorted(ordered[offset:offset + size]))
        offset += size
    return groups


def ambiguous_filters(inventory):
    # Pinned Vitest positional filters are case-insensitive substrings, not
    # regexes. Decide fallback globally so all four runners choose one plan.
    lowered = [name.lower() for name in inventory]
    return (any(not name.isascii() or ":" in name for name in inventory)
            or any(a in b for index, a in enumerate(lowered)
                   for other, b in enumerate(lowered) if index != other))


def load_timings(path):
    if not path.is_file():
        return None
    value = json.loads(path.read_text())
    if (not isinstance(value, dict) or value.get("schema_version") != 1
            or value.get("vitest_version") != "4.1.10"
            or not isinstance(value.get("control_sha256"), dict)
            or not isinstance(value.get("samples"), list) or not value["samples"]
            or not isinstance(value.get("files"), dict) or not value["files"]
            or type(value.get("per_file_overhead_ms")) is not int
            or not 0 <= value["per_file_overhead_ms"] <= MAX_SECONDS * 1000):
        raise ValueError("malformed reviewed timing inventory")
    for sample in value["samples"]:
        if (not isinstance(sample, dict) or not re.fullmatch(r"[0-9a-f]{40}", str(sample.get("source")))
                or any(not re.fullmatch(r"[0-9a-f]{64}", str(sample.get(key)))
                       for key in ("report_sha256", "result_sha256", "inventory_sha256"))):
            raise ValueError("timing sample lacks source/hash provenance")
    for name, record in value["files"].items():
        if (not isinstance(name, str) or name.startswith("/") or ".." in Path(name).parts
                or not isinstance(record, dict) or not re.fullmatch(r"[0-9a-f]{64}", str(record.get("sha256")))
                or not isinstance(record.get("samples_ms"), list) or not record["samples_ms"]):
            raise ValueError("malformed per-file timing history")
        seen = set()
        for row in record["samples_ms"]:
            if (not isinstance(row, list) or len(row) != 2
                    or type(row[0]) is not int or not 0 <= row[0] < len(value["samples"])
                    or row[0] in seen or type(row[1]) is not int or not 0 < row[1] <= MAX_SECONDS * 1000):
                raise ValueError("malformed or duplicate timing sample")
            seen.add(row[0])
    return value


def partition_manifest(inventory, total, source_identity, file_hashes, timings):
    if inventory != sorted(set(inventory)) or not inventory or set(file_hashes) != set(inventory):
        raise ValueError("partition requires the complete unique file/hash inventory")
    reason = None
    if total != 4:
        reason = "native-comparison-group"
    elif timings is None:
        reason = "missing-timing-inventory"
    elif timings["control_sha256"] != source_identity["control_sha256"]:
        reason = "incompatible-test-controls"
    elif ambiguous_filters(inventory):
        reason = "ambiguous-positional-file-filters"
    compatible = ({name for name in inventory if name in timings["files"]
                   and timings["files"][name]["sha256"] == file_hashes[name]}
                  if timings is not None else set())
    if reason is None and not compatible:
        reason = "no-compatible-file-history"
    costs, origins = {}, {}
    if reason is None:
        historical = {name: math.ceil(statistics.median(row[1] for row in record["samples_ms"]))
                      for name, record in timings["files"].items()}
        ranked = sorted(historical.values())
        conservative = ranked[math.ceil(len(ranked) * .95) - 1]
        overhead = timings["per_file_overhead_ms"]
        for name in inventory:
            if name in compatible:
                estimate, origin = historical[name], "matching-file-median"
            else:
                estimate = max(conservative, historical.get(name, 0))
                origin = "changed-file-conservative" if name in historical else "new-file-conservative"
            costs[name], origins[name] = estimate + overhead, origin
        assigned, loads = [[] for _ in range(total)], [0] * total
        for name in sorted(inventory, key=lambda name: (-costs[name], name)):
            index = min(range(total), key=lambda index: (loads[index], index))
            assigned[index].append(name)
            loads[index] += costs[name]
        assigned = [sorted(group) for group in assigned]
    else:
        assigned, loads = native_assignments(inventory, total), None
    return {"schema_version": 1, "algorithm": "native-vitest-4.1.10" if reason else "lpt-median-v1",
            "fallback_reason": reason, "groups": total, "identity": source_identity,
            "inventory_sha256": digest(inventory), "file_sha256": file_hashes,
            "file_costs_ms": costs, "cost_origins": origins, "assignments": assigned,
            "estimated_group_cost_ms": loads,
            "estimate_note": "Median successful JSON file execution time plus median per-file unaccounted shard wall time; estimates are not wall-time predictions."}


def prepare_partition(inventory, total, source_identity, root=ROOT):
    hashes = {name: sha256(root / "web" / name) for name in inventory}
    return partition_manifest(inventory, total, source_identity, hashes, load_timings(root / TIMINGS))


def validate_partition(item):
    manifest = item.get("partition_manifest")
    if (not isinstance(manifest, dict) or item.get("partition_sha256") != digest(manifest)
            or manifest.get("schema_version") != 1 or manifest.get("identity") != item["identity"]
            or manifest.get("groups") != item["shard_total"]
            or manifest.get("inventory_sha256") != digest(item["inventory"])
            or set(manifest.get("file_sha256", {})) != set(item["inventory"])):
        raise ValueError("partition manifest/hash differs from runner identity or inventory")
    assignments = manifest.get("assignments")
    if (not isinstance(assignments, list) or len(assignments) != item["shard_total"]
            or any(not isinstance(group, list) or group != sorted(set(group)) for group in assignments)
            or sorted(name for group in assignments for name in group) != item["inventory"]):
        raise ValueError("partition assignments are incomplete or overlapping")
    assigned = assignments[item["shard_index"] - 1]
    if sorted(item["actual"]["files"]) != assigned:
        raise ValueError("actual files differ from exact assigned partition")
    if manifest.get("algorithm") == "lpt-median-v1":
        if item.get("selected_inventory") != assigned or item.get("selection_returncode") != 0:
            raise ValueError("exact positional-filter selection inventory did not succeed")
    elif manifest.get("algorithm") != "native-vitest-4.1.10":
        raise ValueError("unknown partition algorithm")


def normalize_file(name, web):
    if not isinstance(name, str) or not name:
        raise ValueError("test file name must be a nonempty string")
    path = Path(name)
    resolved = path.resolve() if path.is_absolute() else (web / path).resolve()
    try:
        relative = resolved.relative_to(web.resolve()).as_posix()
    except ValueError as error:
        raise ValueError("test file is outside web root") from error
    if relative == "." or not relative:
        raise ValueError("invalid test file path")
    return relative


def parse_inventory(value, web):
    if not isinstance(value, list) or not value:
        raise ValueError("independent file inventory is empty or malformed")
    files = []
    for item in value:
        if not isinstance(item, dict) or set(item) - {"file", "projectName"}:
            raise ValueError("unexpected inventory record")
        # This repository has one project. A repeated file in multiple projects
        # requires a project-aware coverage schema rather than silently collapsing it.
        if item.get("projectName"):
            raise ValueError("named Vitest projects are not supported by this coverage schema")
        files.append(normalize_file(item.get("file"), web))
    if len(set(files)) != len(files):
        raise ValueError("duplicate files in independent inventory")
    return sorted(files)


ANSI = re.compile(r"\x1b\[[0-9;]*m")


def failure_digest(log_path):
    """Explain a Vitest run that failed although its JSON report says success.

    Vitest can finish every test file and write a successful JSON report, then
    still exit non-zero because a callback ran after the environment was torn
    down — typically a leaked timer calling React setState ("window is not
    defined"). The runner is right to refuse that run, but "exited with 1, even
    if its JSON says success" alone sends the reader into the job log. This
    digest carries the reason into the failure message.
    """
    try:
        text = Path(log_path).read_text(errors="replace")
    except OSError:
        return ""
    lines = [ANSI.sub("", line).rstrip() for line in text.splitlines()]
    starts = [index for index, line in enumerate(lines) if "Uncaught Exception" in line]
    if not starts:
        return ""
    detail = next((line.strip() for line in lines[starts[0] + 1:starts[0] + 6] if line.strip()), "")
    origin = next((line for line in lines[starts[0]:starts[0] + 40] if "originated in" in line), "")
    match = re.search(r'originated in "([^"]+)"', origin)
    parts = [f"{len(starts)} uncaught exception(s) after the test files finished"]
    if detail:
        parts.append(detail)
    if match:
        parts.append(f"originated in {match.group(1)}")
    return "; ".join(parts)


def parse_report(report, web, returncode):
    if returncode != 0:
        raise ValueError(f"Vitest exited with {returncode}, even if its JSON says success")
    if not isinstance(report, dict) or report.get("success") is not True:
        raise ValueError("Vitest did not report success")
    for key in ("numFailedTests", "numFailedTestSuites"):
        if type(report.get(key)) is not int or report[key] != 0:
            raise ValueError(f"nonzero or missing {key}")
    results = report.get("testResults")
    if not isinstance(results, list) or not results:
        raise ValueError("missing actual test file results")
    files = {}
    counts = Counter()
    for result in results:
        if not isinstance(result, dict) or result.get("status") != "passed":
            raise ValueError("failed or malformed test file result")
        name = normalize_file(result.get("name"), web)
        if name in files:
            raise ValueError("duplicate actual test file result")
        assertions = result.get("assertionResults")
        if not isinstance(assertions, list) or not assertions:
            raise ValueError(f"no assertions reported for {name}")
        actual = []
        for assertion in assertions:
            if not isinstance(assertion, dict):
                raise ValueError("malformed assertion")
            parents, title, status = (assertion.get(key) for key in ("ancestorTitles", "title", "status"))
            if (not isinstance(parents, list) or not all(isinstance(p, str) for p in parents)
                    or not isinstance(title, str) or status not in PASS_STATUSES
                    or assertion.get("failureMessages") not in (None, [])):
                raise ValueError("failed, unfinished or malformed assertion")
            actual.append({"ancestorTitles": parents, "title": title, "status": status})
            counts[status] += 1
        files[name] = sorted(actual, key=lambda a: json.dumps(a, sort_keys=True))
    # numTotalTestSuites includes nested describe blocks; it is not a file count.
    expected = {"numTotalTests": sum(counts.values()), "numPassedTests": counts["passed"],
                "numPendingTests": counts["skipped"], "numTodoTests": counts["todo"]}
    for key, total in expected.items():
        if type(report.get(key)) is not int or report[key] != total:
            raise ValueError(f"{key} disagrees with actual assertion results")
    return {"files": files, "counts": dict(sorted(counts.items())),
            "file_count": len(files), "test_count": sum(counts.values())}


def read_text(path):
    try:
        return Path(path).read_text().strip()
    except (OSError, ValueError):
        return None


def optional_command(command):
    try:
        return checked(command)
    except (OSError, subprocess.SubprocessError):
        return None


def host_resources():
    affinity = sorted(os.sched_getaffinity(0)) if hasattr(os, "sched_getaffinity") else None
    meminfo = read_text("/proc/meminfo") or ""
    total = re.search(r"^MemTotal:\s+(\d+) kB$", meminfo, re.MULTILINE)
    available = re.search(r"^MemAvailable:\s+(\d+) kB$", meminfo, re.MULTILINE)
    return {"platform": sys.platform, "logical_cpu_count": os.cpu_count(),
            "affinity_cpu_count": len(affinity) if affinity is not None else None,
            "affinity_cpu_ids": affinity,
            "node_available_parallelism": optional_command(["node", "-p", "require('node:os').availableParallelism()"]),
            "cpu_model": next((line.split(":", 1)[1].strip() for line in (read_text("/proc/cpuinfo") or "").splitlines()
                               if line.startswith("model name")), None),
            "total_ram_bytes": int(total[1]) * 1024 if total else None,
            "available_ram_bytes_at_start": int(available[1]) * 1024 if available else None,
            "cgroup_v2_cpu_max": read_text("/sys/fs/cgroup/cpu.max"),
            "cgroup_v2_memory_max": read_text("/sys/fs/cgroup/memory.max"),
            "cgroup_v1_cpu_quota_us": read_text("/sys/fs/cgroup/cpu/cpu.cfs_quota_us"),
            "cgroup_v1_cpu_period_us": read_text("/sys/fs/cgroup/cpu/cpu.cfs_period_us"),
            "cgroup_v1_memory_limit_bytes": read_text("/sys/fs/cgroup/memory/memory.limit_in_bytes"),
            "node_version": optional_command(["node", "--version"]),
            "bun_version": optional_command(["bun", "--version"])}


def process_sample(root_pid):
    """Read only stat, statm and argv; never read or emit process environments."""
    if not Path("/proc/self/stat").exists():
        return None
    processes = {}
    for path in Path("/proc").iterdir():
        if not path.name.isdecimal():
            continue
        try:
            fields = (path / "stat").read_text().rsplit(")", 1)[1].split()
            rss = int((path / "statm").read_text().split()[1]) * os.sysconf("SC_PAGE_SIZE")
            argv = (path / "cmdline").read_bytes()[:65536].split(b"\0")
            # Vitest 4's fork pool launches these exact entrypoint files. Thread
            # workers share a process and are intentionally not inferred here.
            worker = any(re.search(rb"/vitest/dist/workers/(?:vmForks|forks)\.js$", arg) for arg in argv)
            processes[int(path.name)] = (int(fields[1]), rss, worker)
        except (OSError, ValueError, IndexError):
            continue  # A process can exit between the individual proc reads.
    descendants = {root_pid}
    while True:
        expanded = descendants | {pid for pid, (ppid, _, _) in processes.items() if ppid in descendants}
        if expanded == descendants:
            break
        descendants = expanded
    values = [processes[pid] for pid in descendants if pid in processes]
    return {"process_count": len(values), "rss_sum_bytes": sum(p[1] for p in values),
            "largest_process_rss_bytes": max((p[1] for p in values), default=0),
            "vitest_fork_worker_process_count": sum(p[2] for p in values)}


def parse_gnu_time(path):
    raw = read_text(path)
    if not raw:
        return None
    fields = {}
    for label, output, multiplier in (("User time (seconds)", "user_cpu_seconds", 1),
                                       ("System time (seconds)", "system_cpu_seconds", 1),
                                       ("Maximum resident set size (kbytes)", "max_single_process_rss_bytes", 1024)):
        match = re.search(r"^\s*" + re.escape(label) + r":\s*([0-9.]+)\s*$", raw, re.MULTILINE)
        if match:
            fields[output] = float(match[1]) * multiplier
    return fields or None


def execute(command, cwd, log_path, seconds, sample_path=None, tee=False):
    measured = command
    time_path = log_path.with_suffix(".time.txt")
    if sample_path is not None and sys.platform == "linux" and Path("/usr/bin/time").is_file():
        measured = ["/usr/bin/time", "-v", "-o", str(time_path), "--"] + command
    started = time.monotonic()
    peaks = {"process_count": 0, "rss_sum_bytes": 0, "largest_process_rss_bytes": 0,
             "vitest_fork_worker_process_count": 0}
    samples, timed_out = 0, False
    with log_path.open("w") as log:
        process = subprocess.Popen(measured, cwd=cwd, stdout=subprocess.PIPE if tee else log,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        stream = sample_path.open("w") if sample_path is not None else None
        selector = selectors.DefaultSelector() if tee else None
        decoder = codecs.getincrementaldecoder("utf-8")("replace") if tee else None
        if tee:
            os.set_blocking(process.stdout.fileno(), False)
            selector.register(process.stdout, selectors.EVENT_READ)
        next_sample, drain_deadline = 0, None
        try:
            while process.poll() is None or (tee and selector.get_map()):
                elapsed = time.monotonic() - started
                if process.poll() is None and elapsed > seconds:
                    timed_out = True
                    os.killpg(process.pid, signal.SIGTERM)
                    try:
                        process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        os.killpg(process.pid, signal.SIGKILL)
                if stream is not None and elapsed >= next_sample:
                    sample = process_sample(process.pid)
                    if sample is not None:
                        stream.write(json.dumps({"elapsed_seconds": round(elapsed, 3), **sample}) + "\n")
                        stream.flush()
                        samples += 1
                        for key in peaks:
                            peaks[key] = max(peaks[key], sample[key])
                    next_sample = elapsed + 1
                if tee:
                    for key, _ in selector.select(timeout=0.1):
                        chunk = os.read(key.fd, 65536)
                        if chunk:
                            text = decoder.decode(chunk)
                            log.write(text)
                            log.flush()
                            sys.stdout.write(text)
                            sys.stdout.flush()
                        else:
                            selector.unregister(key.fileobj)
                            tail = decoder.decode(b"", final=True)
                            log.write(tail)
                            sys.stdout.write(tail)
                            sys.stdout.flush()
                    if process.poll() is not None and selector.get_map():
                        drain_deadline = drain_deadline or time.monotonic() + 2
                        if time.monotonic() > drain_deadline:
                            raise ValueError("test output pipe remained open after process exit")
                else:
                    time.sleep(1)
            code = process.wait()
        finally:
            if selector is not None:
                selector.close()
                process.stdout.close()
            if stream is not None:
                stream.close()
    resources = {"wall_seconds": round(time.monotonic() - started, 3), "timed_out": timed_out,
                 "sample_interval_seconds": 1, "sample_count": samples,
                 "sampled_peaks": peaks if samples else None,
                 "sampling_note": "RSS sum includes shared pages per process; 1s sampling can miss short peaks. Fork counts match Vitest entrypoint argv; threads are not inferred.",
                 "gnu_time": parse_gnu_time(time_path) if time_path.exists() else None}
    return code, resources


def shard_value(value):
    if not re.fullmatch(r"[1-9][0-9]*/[1-9][0-9]*", value):
        raise argparse.ArgumentTypeError("shard must be I/N")
    index, total = map(int, value.split("/"))
    if total not in (1, 2, 4) or not 1 <= index <= total:
        raise argparse.ArgumentTypeError("only complete 1, 2 or 4 shard groups are supported")
    return index, total


def run(args):
    output = Path(args.output_dir).resolve()
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        raise ValueError("output directory must be empty; refusing stale artifacts")
    result = {"schema_version": SCHEMA, "ok": False, "started_at": utc_now(),
              "source": args.source, "shard_index": args.shard[0], "shard_total": args.shard[1]}
    code = None
    try:
        result["identity"] = identity(args.source)
        result["host"] = host_resources()
        web = ROOT / "web"
        result["web_root"] = str(web)
        binary = web / "node_modules/.bin/vitest"
        if not binary.is_file():
            raise ValueError("installed local Vitest binary is missing")
        version = json.loads((web / "node_modules/vitest/package.json").read_text())["version"]
        result["vitest_version"] = version
        if version != "4.1.10":
            raise ValueError("revalidate inventory/report contracts before changing Vitest 4.1.10")
        inventory_path = output / "inventory.json"
        listing = [str(binary), "list", "--filesOnly", f"--json={inventory_path}"]
        result["inventory_command"] = listing
        list_code, list_resources = execute(listing, web, output / "inventory.log", 120)
        result["inventory_returncode"] = list_code
        result["inventory_wall_seconds"] = list_resources["wall_seconds"]
        if list_code != 0 or list_resources["timed_out"]:
            raise ValueError("independent unsharded inventory command failed")
        result["inventory"] = parse_inventory(json.loads(inventory_path.read_text()), web)
        manifest = prepare_partition(result["inventory"], args.shard[1], result["identity"])
        result.update(partition_manifest=manifest, partition_sha256=digest(manifest))
        write_json(output / "partition.json", manifest)
        selected = manifest["assignments"][args.shard[0] - 1]
        if not selected:
            raise ValueError("partition is empty; refusing passWithNoTests")
        report_path = output / "vitest-report.json"
        command = ["bun", "run", "test", "--reporter=default", "--reporter=json", f"--outputFile.json={report_path}"]
        if manifest["algorithm"] == "lpt-median-v1":
            filters = [str(web / name) for name in selected]
            selected_path = output / "selected-inventory.json"
            selection = [str(binary), "list", "--filesOnly", f"--json={selected_path}"] + filters
            result["selection_command"] = selection
            selection_code, selection_resources = execute(selection, web, output / "selection.log", 120)
            result["selection_returncode"] = selection_code
            result["selection_wall_seconds"] = selection_resources["wall_seconds"]
            if selection_code != 0 or selection_resources["timed_out"]:
                raise ValueError("exact positional-filter inventory command failed")
            result["selected_inventory"] = parse_inventory(json.loads(selected_path.read_text()), web)
            if result["selected_inventory"] != selected:
                raise ValueError("Vitest positional filters do not select the exact assigned files")
            command += filters
        elif args.shard[1] > 1:
            command.append(f"--shard={args.shard[0]}/{args.shard[1]}")
        result["test_command"] = command
        print(f"Running unchanged frontend tests: shard {args.shard[0]}/{args.shard[1]}", flush=True)
        code, resources = execute(command, web, output / "test.log", MAX_SECONDS, output / "resources.jsonl", tee=True)
        result.update(test_returncode=code, resources=resources)
        if resources["timed_out"]:
            raise ValueError("test invocation exceeded the bounded 25-minute limit")
        try:
            result["actual"] = parse_report(json.loads(report_path.read_text()), web, code)
        except ValueError as error:
            # 不要用 `digest` 作变量名：run() 里还调用了模块级 digest() 函数。
            log_digest = failure_digest(output / "test.log") if code != 0 else ""
            raise ValueError(f"{error} — {log_digest}" if log_digest else error) from None
        if set(result["actual"]["files"]) - set(result["inventory"]):
            raise ValueError("actual tests include files absent from independent inventory")
        validate_partition(result)
        if identity(args.source) != result["identity"]:
            raise ValueError("source identity changed during execution")
        if prepare_partition(result["inventory"], args.shard[1], result["identity"]) != manifest:
            raise ValueError("partition inputs changed during execution")
        result["ok"] = True
    except (ValueError, OSError, KeyError, TypeError, subprocess.SubprocessError) as error:
        result["error"] = f"{type(error).__name__}: {error}"
    finally:
        result["finished_at"] = utc_now()
        write_json(output / "result.json", result)
    print(json.dumps({k: result[k] for k in ("ok", "shard_index", "shard_total", "source", "error") if k in result}), flush=True)
    # Preserve a genuine failing test exit; metadata failures must also fail CI.
    if result["ok"]:
        return 0
    return code if code is not None and 1 <= code <= 255 else (128 - code if code is not None and -127 <= code < 0 else 1)


def validate_artifact(item, source):
    if not isinstance(item, dict) or item.get("schema_version") != SCHEMA or item.get("ok") is not True:
        raise ValueError("failed or malformed runner result")
    if not all(isinstance(item.get(key), dict) for key in ("identity", "actual", "resources")):
        raise ValueError("missing or malformed nested runner metadata")
    if item.get("source") != source or item.get("identity", {}).get("source") != source:
        raise ValueError("runner source differs from requested source")
    value = item["identity"]
    controls = value.get("control_sha256", {})
    partition_controls = value.get("partition_control_sha256", {})
    if (not re.fullmatch(r"[0-9a-f]{40}", source)
            or not isinstance(value.get("web_tree"), str) or not re.fullmatch(r"[0-9a-f]{40}", value["web_tree"])
            or not isinstance(controls, dict)
            or set(controls) not in ({"web/vitest.config.ts", "web/package.json", "web/bun.lock"},
                                     {"web/vitest.config.ts", "web/package.json", "web/bun.lockb"})
            or not all(isinstance(v, str) and re.fullmatch(r"[0-9a-f]{64}", v) for v in controls.values())):
        raise ValueError("missing or malformed source/config/lock identity")
    if (not isinstance(partition_controls, dict) or set(partition_controls) != set(PARTITION_CONTROLS)
            or not re.fullmatch(r"[0-9a-f]{64}", str(partition_controls.get(PARTITION_CONTROLS[0])))
            or (partition_controls[TIMINGS] is not None
                and not re.fullmatch(r"[0-9a-f]{64}", str(partition_controls[TIMINGS])))):
        raise ValueError("missing or malformed partition control identity")
    if any(type(item.get(key)) is not int or item[key] != 0 for key in ("test_returncode", "inventory_returncode")):
        raise ValueError("runner subprocess did not succeed")
    if item.get("vitest_version") != "4.1.10":
        raise ValueError("inconsistent Vitest version")
    if item.get("resources", {}).get("timed_out") is not False:
        raise ValueError("runner timed out or omitted timeout state")
    if not isinstance(item.get("actual", {}).get("files"), dict):
        raise ValueError("missing actual assertion inventory")


def aggregate_results(items, source, groups):
    if not items:
        raise ValueError("no runner result artifacts")
    expected = {(index, total) for total in groups for index in range(1, total + 1)}
    indexed = {}
    reference = None
    inventory = None
    for item in items:
        validate_artifact(item, source)
        key = (item.get("shard_index"), item.get("shard_total"))
        if any(type(part) is not int for part in key) or key not in expected or key in indexed:
            raise ValueError(f"unexpected or duplicate shard result {key}")
        current_inventory = item.get("inventory")
        if (not isinstance(current_inventory, list) or not current_inventory
                or not all(isinstance(p, str) for p in current_inventory)
                or current_inventory != sorted(set(current_inventory))):
            raise ValueError("malformed independent file inventory")
        if reference is None:
            reference, inventory = item["identity"], current_inventory
        if item["identity"] != reference or current_inventory != inventory:
            raise ValueError("source tree, config, lock or full inventory differs between shards")
        validate_partition(item)
        indexed[key] = item
    if set(indexed) != expected:
        raise ValueError(f"missing shard results: {sorted(expected - set(indexed))}")
    combined = {}
    summaries = {}
    for total in groups:
        files, counts, wall_times, cpu_times, resource_details = {}, Counter(), [], [], []
        manifest = indexed[1, total]["partition_manifest"]
        for index in range(1, total + 1):
            item = indexed[index, total]
            if item["partition_manifest"] != manifest:
                raise ValueError("partition manifest differs between shards")
            actual = item["actual"]
            if set(files) & set(actual["files"]):
                raise ValueError(f"test file overlap in {total}-shard group")
            for name, assertions in actual["files"].items():
                if not isinstance(assertions, list) or not assertions:
                    raise ValueError("empty or malformed actual assertions")
                canonical = []
                for assertion in assertions:
                    if (not isinstance(assertion, dict) or set(assertion) != {"ancestorTitles", "title", "status"}
                            or not isinstance(assertion["ancestorTitles"], list)
                            or not all(isinstance(p, str) for p in assertion["ancestorTitles"])
                            or not isinstance(assertion["title"], str) or assertion["status"] not in PASS_STATUSES):
                        raise ValueError("failed or malformed actual assertion")
                    canonical.append(json.dumps(assertion, sort_keys=True))
                    counts[assertion["status"]] += 1
                files[name] = sorted(canonical)
            shard_counts = Counter(a["status"] for assertions in actual["files"].values() for a in assertions)
            if (actual.get("file_count") != len(actual["files"])
                    or actual.get("test_count") != sum(shard_counts.values())
                    or actual.get("counts") != dict(sorted(shard_counts.items()))):
                raise ValueError("runner actual counts disagree with assertion inventory")
            resources = item["resources"]
            wall = resources.get("wall_seconds")
            if type(wall) not in (int, float) or not math.isfinite(wall) or wall <= 0:
                raise ValueError("missing measured test wall time")
            wall_times.append(wall)
            timing = resources.get("gnu_time") or {}
            user, system = timing.get("user_cpu_seconds"), timing.get("system_cpu_seconds")
            if (type(user) in (int, float) and type(system) in (int, float)
                    and math.isfinite(user) and math.isfinite(system) and user >= 0 and system >= 0):
                cpu_times.append(user + system)
            resource_details.append({"index": index, "host": item.get("host"), "resources": resources})
        if set(files) != set(inventory):
            raise ValueError(f"{total}-shard union does not equal full independent inventory")
        combined[total] = files
        summaries[str(total)] = {"files": len(files), "assertions": sum(counts.values()),
                                 "statuses": dict(sorted(counts.items())), "slowest_shard_wall_seconds": max(wall_times),
                                 "sum_shard_wall_seconds": round(sum(wall_times), 3),
                                 "sum_cpu_seconds": round(sum(cpu_times), 3) if len(cpu_times) == total else None,
                                 "partition_sha256": digest(manifest), "partition_algorithm": manifest["algorithm"],
                                 "partition_fallback_reason": manifest["fallback_reason"],
                                 "estimated_group_cost_ms": manifest["estimated_group_cost_ms"],
                                 "per_shard_resources": resource_details}
    first = combined[groups[0]]
    if any(files != first for files in combined.values()):
        raise ValueError("per-file assertion name/status multisets differ between groups")
    return {"schema_version": SCHEMA, "ok": True, "source": source, "identity": reference,
            "groups": summaries, "comparison": "Exact per-file assertion tuple/status multisets; duplicate names retained.",
            "verified_at": utc_now()}


def aggregate(args):
    output = Path(args.artifacts).resolve()
    output.mkdir(parents=True, exist_ok=True)
    acceptance = {"schema_version": SCHEMA, "ok": False, "source": args.source, "verified_at": utc_now()}
    try:
        groups = [int(group) for group in args.groups.split(",")]
        if not groups or len(set(groups)) != len(groups) or set(groups) - {1, 2, 4}:
            raise ValueError("groups must contain distinct values from 1,2,4")
        paths = sorted(output.rglob("result.json"))
        items = []
        for path in paths:
            item = json.loads(path.read_text())
            validate_artifact(item, args.source)
            web = Path(item["web_root"])
            if not web.is_absolute():
                raise ValueError("missing absolute runner web root")
            raw_inventory = parse_inventory(json.loads((path.parent / "inventory.json").read_text()), web)
            raw_actual = parse_report(json.loads((path.parent / "vitest-report.json").read_text()), web, item["test_returncode"])
            if raw_inventory != item.get("inventory") or raw_actual != item.get("actual"):
                raise ValueError("runner result differs from independent raw inventory/report artifacts")
            if json.loads((path.parent / "partition.json").read_text()) != item.get("partition_manifest"):
                raise ValueError("runner partition differs from raw partition artifact")
            if item["partition_manifest"]["algorithm"] == "lpt-median-v1":
                raw_selected = parse_inventory(json.loads((path.parent / "selected-inventory.json").read_text()), web)
                if raw_selected != item.get("selected_inventory"):
                    raise ValueError("runner selection differs from raw selected inventory artifact")
            expected_partition = prepare_partition(raw_inventory, item["shard_total"], identity(args.source))
            if expected_partition != item.get("partition_manifest"):
                raise ValueError("partition is not reproducible from the aggregate checkout")
            items.append(item)
        acceptance = aggregate_results(items, args.source, groups)
        if acceptance["identity"] != identity(args.source):
            raise ValueError("artifact source identity differs from the aggregate checkout")
        acceptance["artifact_paths"] = [str(path.relative_to(output)) for path in paths]
    except (ValueError, OSError, KeyError, TypeError) as error:
        acceptance["ok"] = False
        acceptance["error"] = f"{type(error).__name__}: {error}"
    write_json(output / "acceptance.json", acceptance)
    lines = ["Frontend test coverage and timing", "", f"Source: `{args.source}`", "", f"Accepted: **{acceptance['ok']}**", ""]
    if acceptance["ok"]:
        lines += ["| Shards | Files | Assertions | Slowest shard (s) | Total runner time (s) | Total CPU (s) |",
                  "| --- | --- | --- | --- | --- | --- |"]
        for group, result in acceptance["groups"].items():
            lines.append(f"| {group} | {result['files']} | {result['assertions']} | {result['slowest_shard_wall_seconds']} | {result['sum_shard_wall_seconds']} | {result['sum_cpu_seconds']} |")
        lines += ["", acceptance["comparison"], "", "Times measure test invocations, excluding dependency installation and runner queueing.",
                  "Resource sampling is available per shard; RSS sums count shared pages per process. No test worker or assertion settings were changed."]
        for group, result in acceptance["groups"].items():
            lines += ["", f"Group {group}: `{result['partition_algorithm']}`; manifest `{result['partition_sha256']}`; "
                      f"fallback `{result['partition_fallback_reason']}`."]
        lines += ["", "| Group / shard | Affinity CPUs | Total RAM (GiB) | Peak fork workers | Sampled process RSS sum peak (MiB) | GNU time max process RSS (MiB) |",
                  "| --- | --- | --- | --- | --- | --- |"]
        for group, result in acceptance["groups"].items():
            for detail in result["per_shard_resources"]:
                host, resources = detail.get("host") or {}, detail["resources"]
                peaks, timing = resources.get("sampled_peaks") or {}, resources.get("gnu_time") or {}
                def units(value, divisor):
                    return round(value / divisor, 2) if isinstance(value, (int, float)) else "unavailable"
                lines.append(f"| {group} / {detail['index']} | {host.get('affinity_cpu_count')} | {units(host.get('total_ram_bytes'), 2 ** 30)} | {peaks.get('vitest_fork_worker_process_count')} | {units(peaks.get('rss_sum_bytes'), 2 ** 20)} | {units(timing.get('max_single_process_rss_bytes'), 2 ** 20)} |")
    else:
        lines.append(acceptance["error"])
    (output / "summary.md").write_text("\n".join(lines) + "\n")
    print("\n".join(lines), flush=True)
    return 0 if acceptance["ok"] else 1


def collect_timings(args):
    """Produce reviewable costs from complete successful historical four-way runs.

    Nothing is learned or overwritten during CI execution. The recorded web
    tree must exist locally to bind historical test contents. Temporary PR
    merge commits may already be gone; their source SHA remains provenance.
    """
    paths = sorted(path for directory in args.artifacts for path in Path(directory).rglob("result.json"))
    samples, files, overheads, runs = [], {}, [], {}
    reference = inventory = historical_hashes = None
    for path in paths:
        item = json.loads(path.read_text())
        if item.get("shard_total") != 4:
            continue
        if (item.get("ok") is not True or item.get("vitest_version") != "4.1.10"
                or item.get("test_returncode") != 0 or item.get("inventory_returncode") != 0
                or not re.fullmatch(r"[0-9a-f]{40}", str(item.get("source")))
                or item.get("identity", {}).get("source") != item.get("source")
                or item.get("resources", {}).get("timed_out") is not False):
            raise ValueError("timing inputs must be genuine successful four-way results")
        match = re.fullmatch(r"frontend-ci-([0-9]+)-four-([1-4])-([0-9]+)", path.parent.name)
        if not match or int(match[2]) != item["shard_index"]:
            raise ValueError("timing artifact lacks run/shard/artifact provenance")
        historical_web = Path(item["web_root"])
        def historical_relative(name):
            # Historical /home paths can trigger macOS automount lookups.
            # Check their lexical boundary before mapping to this checkout.
            candidate = Path(name)
            relative = candidate.relative_to(historical_web) if candidate.is_absolute() else candidate
            if ".." in relative.parts:
                raise ValueError("historical test file escapes its web root")
            return relative.as_posix()
        raw_report = json.loads((path.parent / "vitest-report.json").read_text())
        raw_list = json.loads((path.parent / "inventory.json").read_text())
        for result in raw_report["testResults"]:
            result["name"] = historical_relative(result["name"])
        for entry in raw_list:
            entry["file"] = historical_relative(entry["file"])
        actual = parse_report(raw_report, ROOT / "web", 0)
        raw_inventory = parse_inventory(raw_list, ROOT / "web")
        current = {key: item["identity"][key] for key in ("web_tree", "control_sha256")}
        if reference is None:
            reference, inventory = current, raw_inventory
            if (not re.fullmatch(r"[0-9a-f]{40}", str(current["web_tree"]))
                    or checked(["git", "cat-file", "-t", current["web_tree"]]) != "tree"):
                raise ValueError("historical web tree is unavailable")
            queries = "".join(f"{current['web_tree']}:{name}\n" for name in inventory)
            raw = subprocess.check_output(["git", "cat-file", "--batch"], cwd=ROOT, input=queries.encode())
            historical_hashes, offset = {}, 0
            for name in inventory:
                end = raw.index(b"\n", offset)
                header = raw[offset:end].split()
                if len(header) != 3 or header[1] != b"blob":
                    raise ValueError("historical test file is not a Git blob")
                size = int(header[2])
                content = raw[end + 1:end + 1 + size]
                if len(content) != size or raw[end + 1 + size:end + 2 + size] != b"\n":
                    raise ValueError("incomplete historical Git blob")
                historical_hashes[name] = hashlib.sha256(content).hexdigest()
                offset = end + 2 + size
            if offset != len(raw):
                raise ValueError("unexpected historical Git blob data")
        if current != reference or raw_inventory != inventory or actual != item["actual"]:
            raise ValueError("timing inputs differ in source tree/controls/inventory or raw assertions")
        run = runs.setdefault(match[1], {})
        if match[2] in run:
            raise ValueError("duplicate historical run/shard")
        run[match[2]] = set(actual["files"])
        sample_index = len(samples)
        sample = {"run_id": match[1], "artifact_id": match[3], "artifact_name": path.parent.name,
                  "source": item["source"], "shard_index": item["shard_index"],
                  "result_sha256": sha256(path), "inventory_sha256": sha256(path.parent / "inventory.json"),
                  "report_sha256": sha256(path.parent / "vitest-report.json")}
        duration_sum = 0
        for result in raw_report["testResults"]:
            name = result["name"]
            duration = result["endTime"] - result["startTime"]
            if not math.isfinite(duration) or not 0 < duration <= MAX_SECONDS * 1000:
                raise ValueError("invalid historical file timing")
            duration_ms = math.ceil(duration)
            duration_sum += duration_ms
            file_hash = historical_hashes[name]
            record = files.setdefault(name, {"sha256": file_hash, "samples_ms": []})
            if record["sha256"] != file_hash:
                raise ValueError("collect one compatible web tree at a time")
            record["samples_ms"].append([sample_index, duration_ms])
        wall_ms = math.ceil(item["resources"]["wall_seconds"] * 1000)
        sample.update(wall_ms=wall_ms, sum_file_execution_ms=duration_sum)
        overheads.append(max(0, wall_ms - duration_sum) / len(actual["files"]))
        samples.append(sample)
    if not samples:
        raise ValueError("no successful timing samples")
    for run in runs.values():
        if (set(run) != {"1", "2", "3", "4"}
                or sorted(name for shard in run.values() for name in shard) != inventory):
            raise ValueError("historical run is incomplete or overlapping")
    value = {"schema_version": 1, "vitest_version": "4.1.10", **reference,
             "measurement": "JSON endTime-startTime milliseconds; common overhead is median max(0, shard wall-sum file execution)/file count. Estimates are not wall-time predictions.",
             "per_file_overhead_ms": math.ceil(statistics.median(overheads)),
             "samples": samples, "files": dict(sorted(files.items()))}
    write_json(Path(args.output), value)
    load_timings(Path(args.output))
    print(json.dumps({"output": args.output, "sha256": sha256(Path(args.output)),
                      "samples": len(samples), "files": len(files)}))
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    runner = commands.add_parser("run")
    runner.add_argument("--shard", required=True, type=shard_value)
    runner.add_argument("--source", required=True)
    runner.add_argument("--output-dir", required=True)
    aggregator = commands.add_parser("aggregate")
    aggregator.add_argument("--artifacts", required=True)
    aggregator.add_argument("--source", required=True)
    aggregator.add_argument("--groups", required=True)
    collector = commands.add_parser("collect-timings")
    collector.add_argument("--artifacts", required=True, nargs="+")
    collector.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        return {"run": run, "aggregate": aggregate, "collect-timings": collect_timings}[args.command](args)
    except (ValueError, OSError, KeyError, TypeError, subprocess.SubprocessError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    sys.exit(main())
