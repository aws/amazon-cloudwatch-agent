import json
import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


SCRIPT = Path(__file__).parents[1] / "run-ec2-integration-pair.py"


def case(name: str, wip: bool = False) -> dict:
    return {
        "testName": name,
        "test_dir": f"./test/{name}",
        "terraform_dir": "terraform/ec2/linux",
        "agentStartCommand": "start agent",
        "ami": "ami-test",
        "arc": "amd64",
        "binaryName": "amazon-cloudwatch-agent.rpm",
        "caCertPath": "",
        "instanceType": "t3.micro",
        "excludedTests": "",
        "installAgentCommand": "install agent",
        "selinux_branch": "",
        "os": name,
        "username": "ec2-user",
        "wip": wip,
    }


class RunEC2IntegrationPairTest(unittest.TestCase):
    def setUp(self):
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary_directory.name)
        self.source = self.root / "source"
        (self.source / "terraform/ec2/linux").mkdir(parents=True)
        self.fake_log = self.root / "fake-terraform.log"
        self.fake_terraform = self.root / "terraform"
        self.fake_terraform.write_text(
            textwrap.dedent(
                """\
                #!/usr/bin/env python3
                import os
                import sys
                import time

                command = sys.argv[1]
                name = next(
                    (
                        arg.split("=", 2)[2]
                        for arg in sys.argv
                        if arg.startswith("-var=test_name=")
                    ),
                    "",
                )
                with open(os.environ["FAKE_TF_LOG"], "a", encoding="utf-8") as log:
                    log.write(f"{time.monotonic()}|{os.getcwd()}|{command}|{name}\\n")
                if command == "apply":
                    time.sleep(0.4)
                    if name == "fail":
                        sys.exit(1)
                sys.exit(0)
                """
            ),
            encoding="utf-8",
        )
        self.fake_terraform.chmod(0o755)

    def tearDown(self):
        self.temporary_directory.cleanup()

    def run_pair(self, cases: list[dict]) -> subprocess.CompletedProcess:
        environment = os.environ.copy()
        environment.update(
            {
                "TERRAFORM_BIN": str(self.fake_terraform),
                "FAKE_TF_LOG": str(self.fake_log),
                "PAIR_LAUNCH_DELAY_SECONDS": "0",
                "TERRAFORM_INIT_TIMEOUT_SECONDS": "10",
                "TERRAFORM_APPLY_TIMEOUT_SECONDS": "10",
                "TERRAFORM_DESTROY_TIMEOUT_SECONDS": "10",
                "INPUT_BUILD_ID": "sha",
                "INPUT_TEST_REPO_URL": "https://example.test/repo.git",
                "INPUT_TEST_REPO_BRANCH": "main",
                "INPUT_REGION": "us-west-2",
                "INPUT_S3_BUCKET": "bucket",
            }
        )
        pair = {"pairId": "test", "pairName": "test pair", "cases": cases}
        return subprocess.run(
            [
                str(SCRIPT),
                "--pair-json",
                json.dumps(pair),
                "--source-dir",
                str(self.source),
                "--work-root",
                str(self.root / "work"),
                "--default-terraform-dir",
                "terraform/ec2/linux",
            ],
            env=environment,
            text=True,
            capture_output=True,
        )

    def test_runs_both_apply_commands_concurrently_in_isolated_directories(self):
        completed = self.run_pair([case("first"), case("second")])

        self.assertEqual(0, completed.returncode, completed.stdout + completed.stderr)
        entries = [
            line.split("|")
            for line in self.fake_log.read_text(encoding="utf-8").splitlines()
            if "|apply|" in line
        ]
        self.assertEqual(2, len(entries))
        self.assertNotEqual(entries[0][1], entries[1][1])
        self.assertLess(abs(float(entries[0][0]) - float(entries[1][0])), 0.25)

    def test_overrules_wip_failure(self):
        completed = self.run_pair([case("pass"), case("fail", wip=True)])

        self.assertEqual(0, completed.returncode, completed.stdout + completed.stderr)
        self.assertIn("WIP test failed and was overruled", completed.stdout)

    def test_fails_non_wip_failure(self):
        completed = self.run_pair([case("pass"), case("fail")])

        self.assertEqual(1, completed.returncode, completed.stdout + completed.stderr)
        self.assertIn("integration test failed", completed.stdout)


if __name__ == "__main__":
    unittest.main()
