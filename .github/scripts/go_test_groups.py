"""Run the complete Go suite in disjoint CI groups without changing test flags."""

import argparse
import hashlib
import json
import re
import subprocess
import sys


HEAVY = ("internal/handlers", "internal/services")
GROUPS = ("handlers-1", "handlers-2", "services-1", "services-2", "other")
FLAGS = ("-race", "-count=1", "-timeout=1000s", "-p=4")
NAME = re.compile(r"(?:Test|Example|Fuzz)\w*\Z")


def capture(args):
    return subprocess.check_output(args, text=True, encoding="utf-8")


def package_groups(packages):
    if len(packages) != len(set(packages)):
        raise ValueError("duplicate Go package")
    heavy = {}
    for suffix in HEAVY:
        matches = [p for p in packages if p.endswith("/" + suffix)]
        if len(matches) != 1:
            raise ValueError(f"expected exactly one package for {suffix}: {matches}")
        heavy[suffix.rsplit("/", 1)[-1]] = matches[0]
    other = sorted(set(packages) - set(heavy.values()))
    if not other:
        raise ValueError("remaining package group is empty")
    if set(other) | set(heavy.values()) != set(packages):
        raise ValueError("package coverage mismatch")
    return heavy, other


def list_names(output):
    # -list includes benchmarks, which the original go test command did not run.
    names = [line for line in output.splitlines() if NAME.fullmatch(line)]
    if len(names) != len(set(names)):
        raise ValueError("duplicate top-level test name")
    if not names:
        raise ValueError("no runnable tests, examples or fuzz seeds discovered")
    return sorted(names)


def partition(names):
    if len(names) != len(set(names)):
        raise ValueError("duplicate inventory entry")
    groups = [sorted(names)[i::2] for i in range(2)]
    flat = [name for group in groups for name in group]
    if len(flat) != len(names) or set(flat) != set(names):
        raise ValueError("test coverage mismatch")
    if not all(groups):
        raise ValueError("empty test shard")
    return groups


def run_pattern(names):
    if not names or any(not NAME.fullmatch(name) for name in names):
        raise ValueError("invalid selected test name")
    # Exact top-level names: all their subtests and fuzz seeds remain enabled.
    return "^(" + "|".join(re.escape(name) for name in names) + ")$"


def check_finished(expected, finished):
    if set(expected) != set(finished):
        missing = sorted(set(expected) - set(finished))
        extra = sorted(set(finished) - set(expected))
        raise ValueError(f"executed test coverage mismatch: missing={missing}, extra={extra}")


def run_selected(package, selected):
    command = ["go", "test", *FLAGS, "-json", "-run", run_pattern(selected), package]
    finished = set()
    process = subprocess.Popen(command, stdout=subprocess.PIPE, text=True, encoding="utf-8")
    for line in process.stdout:
        sys.stdout.write(line)
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        name = event.get("Test", "")
        if name and "/" not in name and event.get("Action") in ("pass", "skip"):
            finished.add(name)
    result = process.wait()
    if result:
        raise subprocess.CalledProcessError(result, command)
    check_finished(selected, finished)
    print(f"Verified completed top-level tests: {len(finished)}", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("group", choices=GROUPS)
    parser.add_argument("--inventory-only", action="store_true")
    args = parser.parse_args()
    packages = capture(["go", "list", "./..."]).splitlines()
    heavy, other = package_groups(packages)
    if args.group == "other":
        print(json.dumps({"group": "other", "packages": other}), flush=True)
        if not args.inventory_only:
            subprocess.run(["go", "test", *FLAGS, *other], check=True)
        return
    family, index = args.group.split("-")
    package = heavy[family]
    names = list_names(capture(["go", "test", *FLAGS, "-list", ".", package]))
    shards = partition(names)
    selected = shards[int(index) - 1]
    digest = hashlib.sha256("\n".join(names).encode()).hexdigest()
    print(json.dumps({"group": args.group, "package": package, "total": len(names),
                      "shard_sizes": [len(s) for s in shards], "selected": len(selected),
                      "inventory_sha256": digest}), flush=True)
    if not args.inventory_only:
        run_selected(package, selected)


if __name__ == "__main__":
    main()
