import unittest
import re
from pathlib import Path

import wait_release_checks as gate


REPOSITORY = "example/project"
SHA = "a" * 40
FINGERPRINT = "SHA256:" + "A" * 43


def run(workflow="security.yml", **changes):
    result = {"id": 10, "head_sha": SHA, "head_branch": "dev", "event": "push",
              "path": ".github/workflows/" + workflow, "status": "completed",
              "conclusion": "success", "head_repository": {"full_name": REPOSITORY}}
    result.update(changes)
    return result


class FakeAPI:
    def __init__(self, runs, head=SHA):
        self.runs = runs
        self.head = head
        self.paths = []

    def get(self, path):
        self.paths.append(path)
        if path.startswith("/git/ref/"):
            return {"object": {"sha": self.head}}
        return {"workflow_runs": self.runs}


class ReleaseGateTest(unittest.TestCase):
    def test_workflow_contract_prevents_missing_gate_or_staging_cycle(self):
        gate.validate_workflows("dev", ["security.yml"])
        gate.validate_workflows("prod", ["ci.yml", "security.yml"])
        for branch, workflows in [("dev", ["ci.yml", "security.yml"]),
                                  ("prod", ["ci.yml"]), ("prod", ["security.yml"])]:
            with self.subTest(branch=branch, workflows=workflows), self.assertRaises(gate.GateError):
                gate.validate_workflows(branch, workflows)

    def test_valid_target(self):
        gate.validate_target(REPOSITORY, SHA, "dev", "refs/heads/dev", FINGERPRINT)
        gate.validate_target(REPOSITORY, SHA, "prod", "refs/heads/prod", FINGERPRINT)

    def test_invalid_identity_or_missing_trusted_fingerprint_fails_closed(self):
        for args in [
            ("../project", SHA, "dev", "refs/heads/dev", FINGERPRINT),
            (REPOSITORY, "main", "dev", "refs/heads/dev", FINGERPRINT),
            (REPOSITORY, SHA, "prod", "refs/heads/dev", FINGERPRINT),
            (REPOSITORY, SHA, "prod", "refs/tags/prod", FINGERPRINT),
            (REPOSITORY, SHA, "dev", "refs/heads/dev", ""),
            (REPOSITORY, SHA, "dev", "refs/heads/dev", "SHA256:guessed"),
        ]:
            with self.subTest(args=args), self.assertRaises(gate.GateError):
                gate.validate_target(*args)

    def test_latest_matching_run_excludes_wrong_revision_branch_event_repo_workflow(self):
        good = run()
        candidates = [good, run(id=11, head_sha="b" * 40),
                      run(id=12, head_branch="prod"), run(id=13, event="pull_request"),
                      run(id=14, head_repository={"full_name": "attacker/project"}),
                      run(id=15, path=".github/workflows/other.yml")]
        self.assertEqual(gate.latest_run(candidates, REPOSITORY, SHA, "dev", "security.yml"), good)

    def test_newer_failed_run_cannot_reuse_older_success(self):
        api = FakeAPI([run(id=10), run(id=11, conclusion="failure")])
        with self.assertRaises(gate.GateError):
            gate.wait_checks(api, REPOSITORY, SHA, "dev", ["security.yml"])

    def test_success_requires_current_branch_head(self):
        api = FakeAPI([run()])
        gate.wait_checks(api, REPOSITORY, SHA, "dev", ["security.yml"])
        self.assertEqual(api.paths[-1], "/git/ref/heads/dev")
        with self.assertRaises(gate.GateError):
            gate.wait_checks(FakeAPI([run()], head="b" * 40), REPOSITORY, SHA, "dev", ["security.yml"])

    def test_all_non_success_conclusions_block(self):
        for conclusion in ["failure", "cancelled", "skipped", "timed_out", "neutral", None]:
            with self.subTest(conclusion=conclusion), self.assertRaises(gate.GateError):
                gate.wait_checks(FakeAPI([run(conclusion=conclusion)]), REPOSITORY, SHA,
                                 "dev", ["security.yml"])

    def test_pending_and_absent_runs_have_a_bounded_wait(self):
        for runs in [[], [run(status="in_progress", conclusion=None)]]:
            ticks = iter([0, 0, 1])
            sleeps = []
            with self.assertRaises(gate.GateError):
                gate.wait_checks(FakeAPI(runs), REPOSITORY, SHA, "dev", ["security.yml"],
                                 timeout=1, monotonic=lambda: next(ticks), sleep=sleeps.append)
            self.assertEqual(sleeps, [1])

    def test_pending_run_can_finish_without_real_sleep(self):
        api = FakeAPI([run(status="in_progress", conclusion=None)])

        def finish(_):
            api.runs = [run()]

        gate.wait_checks(api, REPOSITORY, SHA, "dev", ["security.yml"], sleep=finish)

    def test_production_requires_both_exact_sha_workflows(self):
        runs = [run(head_branch="prod"), run("ci.yml", head_branch="prod")]
        gate.wait_checks(FakeAPI(runs), REPOSITORY, SHA, "prod", ["ci.yml", "security.yml"])
        runs[1]["conclusion"] = "failure"
        with self.assertRaises(gate.GateError):
            gate.wait_checks(FakeAPI(runs), REPOSITORY, SHA, "prod", ["ci.yml", "security.yml"])


class WorkflowDependencyTests(unittest.TestCase):
    def test_image_publication_observes_failed_validation_jobs(self):
        workflow = (Path(__file__).resolve().parents[1] / "workflows" / "ci.yml").read_text(encoding="utf-8")
        build = workflow.split("\n  docker-build:", 1)[1].split("\n  deploy-staging:", 1)[0]
        match = re.search(r"^    needs: \[([^\]]+)\]", build, re.MULTILINE)
        self.assertIsNotNone(match)
        dependencies = {name.strip() for name in match.group(1).split(",")}
        # Tests/bundle can be skipped transitively after lint failure. The
        # publisher must observe the lint job itself, not infer its outcome.
        self.assertTrue({"backend-test", "frontend-lint", "frontend-test",
                         "frontend-bundle", "release-policy-tests"} <= dependencies)
        self.assertNotIn("deploy-staging", dependencies)
        self.assertNotIn("summary", dependencies)


if __name__ == "__main__":
    unittest.main()
