"""Fail closed unless the exact release revision passed the required push workflows."""

import argparse
import json
import os
import re
import sys
import time
import urllib.parse
import urllib.request


class GateError(Exception):
    pass


def validate_workflows(branch, workflows):
    required = {"security.yml"} if branch == "dev" else {"ci.yml", "security.yml"}
    if set(workflows) != required:
        raise GateError("The complete release workflow gate is required")


def validate_target(repository, sha, branch, ref, fingerprint):
    if (not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+", repository)
            or repository.endswith(("/.", "/.."))):
        raise GateError("Invalid repository")
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise GateError("Invalid release SHA")
    if branch not in ("dev", "prod") or ref != "refs/heads/" + branch:
        raise GateError("Release must originate from its deployment branch")
    if not re.fullmatch(r"SHA256:[A-Za-z0-9+/]{43}=?", fingerprint):
        raise GateError("A trusted VPS_HOST_FINGERPRINT environment secret is required")


class GitHub:
    def __init__(self, repository, token):
        self.base = "https://api.github.com/repos/" + repository
        self.token = token

    def get(self, path):
        request = urllib.request.Request(self.base + path, headers={
            "Authorization": "Bearer " + self.token,
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        })
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.load(response)
        except Exception:
            # Do not print request headers, credentials or response bodies.
            raise GateError("GitHub release-check API request failed") from None


def latest_run(runs, repository, sha, branch, workflow):
    expected_path = ".github/workflows/" + workflow
    matching = [run for run in runs if
                run.get("head_sha") == sha and run.get("head_branch") == branch
                and run.get("event") == "push" and run.get("path") == expected_path
                and (run.get("head_repository") or {}).get("full_name") == repository]
    return max(matching, key=lambda run: run["id"], default=None)


def wait_checks(api, repository, sha, branch, workflows, timeout=1800,
                monotonic=time.monotonic, sleep=time.sleep):
    deadline = monotonic() + timeout
    while True:
        pending = []
        for workflow in workflows:
            query = urllib.parse.urlencode({"head_sha": sha, "branch": branch,
                                            "event": "push", "per_page": 100})
            payload = api.get("/actions/workflows/" + workflow + "/runs?" + query)
            run = latest_run(payload.get("workflow_runs", []), repository, sha, branch, workflow)
            if run is None or run.get("status") != "completed":
                pending.append(workflow)
            elif run.get("conclusion") != "success":
                raise GateError(workflow + " did not succeed for the release SHA")
        if not pending:
            # A queued older deploy must not roll back a newer branch revision.
            current = api.get("/git/ref/heads/" + branch)
            if (current.get("object") or {}).get("sha") != sha:
                raise GateError("Deployment branch advanced beyond the release SHA")
            return
        remaining = deadline - monotonic()
        if remaining <= 0:
            raise GateError("Timed out waiting for exact-SHA release checks")
        sleep(min(10, remaining))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--branch", choices=("dev", "prod"), required=True)
    parser.add_argument("--workflow", choices=("ci.yml", "security.yml"), action="append", required=True)
    args = parser.parse_args()
    try:
        repository = os.environ.get("GITHUB_REPOSITORY", "")
        sha = os.environ.get("GITHUB_SHA", "")
        validate_target(repository, sha, args.branch, os.environ.get("GITHUB_REF", ""),
                        os.environ.get("VPS_HOST_FINGERPRINT", ""))
        validate_workflows(args.branch, args.workflow)
        token = os.environ.get("GH_TOKEN", "")
        if not token:
            raise GateError("A release-check API token is required")
        wait_checks(GitHub(repository, token), repository, sha, args.branch, args.workflow)
        print("Exact-SHA release checks passed")
    except GateError as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
