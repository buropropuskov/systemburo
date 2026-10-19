import unittest

import go_test_groups as runner


class GoTestGroupsTest(unittest.TestCase):
    def test_packages_cover_original_inventory(self):
        packages = ["systemburo/internal/handlers", "systemburo/internal/services", "systemburo/cmd/api"]
        heavy, other = runner.package_groups(packages)
        self.assertEqual(set(packages), set(heavy.values()) | set(other))
        self.assertFalse(set(heavy.values()) & set(other))

    def test_missing_or_duplicate_packages_fail(self):
        for packages in ([], ["systemburo/internal/handlers"] * 2):
            with self.assertRaises(ValueError):
                runner.package_groups(packages)

    def test_listing_keeps_tests_examples_fuzz_but_not_benchmarks(self):
        names = runner.list_names("TestA\nExample\nExampleThing\nFuzzDecode\nBenchmarkDecode\nok\tmodule 0.1s\n")
        self.assertEqual(names, ["Example", "ExampleThing", "FuzzDecode", "TestA"])

    def test_listing_rejects_empty_or_duplicate(self):
        for output in ("ok module", "TestA\nTestA\n"):
            with self.assertRaises(ValueError):
                runner.list_names(output)

    def test_partitions_are_complete_disjoint_and_deterministic(self):
        names = ["TestZ", "FuzzInput", "TestA", "ExampleX", "TestB"]
        a, b = runner.partition(names)
        self.assertEqual(set(names), set(a) | set(b))
        self.assertFalse(set(a) & set(b))
        self.assertEqual([a, b], runner.partition(list(reversed(names))))

    def test_exact_pattern_does_not_match_neighboring_names(self):
        import re
        pattern = re.compile(runner.run_pattern(["TestA", "FuzzDecode", "ExampleX"]))
        self.assertIsNotNone(pattern.fullmatch("TestA"))
        self.assertIsNone(pattern.fullmatch("TestAB"))

    def test_incomplete_or_extra_execution_fails(self):
        runner.check_finished(["TestA", "TestB"], {"TestA", "TestB"})
        for finished in ({"TestA"}, {"TestA", "TestB", "TestC"}):
            with self.assertRaises(ValueError):
                runner.check_finished(["TestA", "TestB"], finished)

    def test_command_keeps_flags_and_nested_results(self):
        import io
        import json
        from unittest.mock import patch
        events = [{"Test": name, "Action": "pass"} for name in ("TestA/sub", "TestA", "Example", "FuzzX/seed#0", "FuzzX")]
        with patch.object(runner.subprocess, "Popen") as start, patch("sys.stdout", new=io.StringIO()):
            start.return_value.stdout = [json.dumps(event) + "\n" for event in events]
            start.return_value.wait.return_value = 0
            runner.run_selected("module/internal/handlers", ["TestA", "Example", "FuzzX"])
            command = start.call_args.args[0]
            for flag in runner.FLAGS:
                self.assertIn(flag, command)
            self.assertIn("-json", command)

    def test_failed_go_command_is_not_hidden(self):
        import io
        import subprocess
        from unittest.mock import patch
        with patch.object(runner.subprocess, "Popen") as start, patch("sys.stdout", new=io.StringIO()):
            start.return_value.stdout = []
            start.return_value.wait.return_value = 1
            with self.assertRaises(subprocess.CalledProcessError):
                runner.run_selected("module/internal/handlers", ["TestA"])


if __name__ == "__main__":
    unittest.main()
