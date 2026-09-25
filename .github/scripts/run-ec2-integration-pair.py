#!/usr/bin/env python3

import argparse
import concurrent.futures
import json
import os
import shutil
import signal
import subprocess
import sys
import threading
import time
import traceback
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any


ACTIVE_PROCESSES: set[subprocess.Popen[Any]] = set()
ACTIVE_PROCESSES_LOCK = threading.Lock()
CANCELLED = threading.Event()


@dataclass
class CaseResult:
    name: str
    wip: bool
    init_status: int | None = None
    apply_status: int | None = None
    destroy_status: int | None = None
    init_seconds: float = 0
    apply_seconds: float = 0
    destroy_seconds: float = 0
    total_seconds: float = 0
    timed_out: bool = False
    cancelled: bool = False

    @property
    def execution_failed(self) -> bool:
        return self.init_status not in (None, 0) or self.apply_status not in (None, 0)

    @property
    def cleanup_failed(self) -> bool:
        return self.destroy_status not in (None, 0)


def _terminate_process(process: subprocess.Popen[Any]) -> None:
    if process.poll() is not None:
        return
    try:
        os.killpg(process.pid, signal.SIGTERM)
        process.wait(timeout=10)
    except (ProcessLookupError, subprocess.TimeoutExpired):
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


def _handle_signal(signum: int, _frame: Any) -> None:
    CANCELLED.set()
    with ACTIVE_PROCESSES_LOCK:
        processes = list(ACTIVE_PROCESSES)
    for process in processes:
        _terminate_process(process)
    raise KeyboardInterrupt(f"received signal {signum}")


def _run_command(
    command: list[str],
    cwd: Path,
    log_file: Any,
    timeout_seconds: int,
    environment: dict[str, str],
) -> tuple[int, bool, float]:
    started_at = time.monotonic()
    if CANCELLED.is_set():
        return 130, False, 0

    process = subprocess.Popen(
        command,
        cwd=cwd,
        env=environment,
        stdout=log_file,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    with ACTIVE_PROCESSES_LOCK:
        ACTIVE_PROCESSES.add(process)
    try:
        status = process.wait(timeout=timeout_seconds)
        return status, False, time.monotonic() - started_at
    except subprocess.TimeoutExpired:
        _terminate_process(process)
        return 124, True, time.monotonic() - started_at
    finally:
        with ACTIVE_PROCESSES_LOCK:
            ACTIVE_PROCESSES.discard(process)


def _case_name(case: dict[str, Any], index: int) -> str:
    return str(case.get("testName") or case.get("test_dir") or f"case-{index}")


def normalize_matrix_entry(entry: dict[str, Any]) -> dict[str, Any]:
    """Accept a paired PR matrix entry or a legacy single-case matrix entry."""
    cases = entry.get("cases")
    if cases is None:
        name = _case_name(entry, 0)
        return {
            "pairId": "legacy-single",
            "pairName": name,
            "cases": [entry],
        }
    if not isinstance(cases, list) or not 1 <= len(cases) <= 2:
        raise ValueError("matrix entry must contain one or two cases")
    return entry


def _case_working_directory(
    case_root: Path, case: dict[str, Any], default_terraform_dir: str
) -> Path:
    relative_path = Path(str(case.get("terraform_dir") or default_terraform_dir))
    if relative_path.is_absolute() or ".." in relative_path.parts:
        raise ValueError(f"unsafe Terraform directory: {relative_path}")

    repository_root = (case_root / "test-repo").resolve()
    terraform_directory = (repository_root / relative_path).resolve()
    if (
        repository_root not in terraform_directory.parents
        and terraform_directory != repository_root
    ):
        raise ValueError(f"Terraform directory escapes the case workspace: {relative_path}")
    return terraform_directory


def _terraform_apply_command(
    terraform: str,
    case: dict[str, Any],
    shared: argparse.Namespace,
) -> list[str]:
    def value(name: str) -> str:
        raw_value = case.get(name, "")
        return "" if raw_value is None else str(raw_value)

    variables = {
        "agent_start": value("agentStartCommand"),
        "ami": value("ami"),
        "arc": value("arc"),
        "binary_name": value("binaryName"),
        "ca_cert_path": value("caCertPath"),
        "cwa_github_sha": shared.build_id,
        "ec2_instance_type": value("instanceType"),
        "excluded_tests": f"'{value('excludedTests')}'",
        "github_test_repo": shared.test_repo_url,
        "github_test_repo_branch": shared.test_repo_branch,
        "install_agent": value("installAgentCommand"),
        "is_selinux_test": str(shared.is_selinux_test).lower(),
        "selinux_branch": value("selinux_branch"),
        "local_stack_host_name": shared.localstack_host,
        "plugin_tests": f"'{shared.plugins}'",
        "region": shared.region,
        "s3_bucket": shared.s3_bucket,
        "ssh_key_name": os.environ.get("KEY_NAME", ""),
        "ssh_key_value": os.environ.get("PRIVATE_KEY", ""),
        "test_dir": value("test_dir"),
        "test_name": value("os"),
        "is_onprem": str(shared.is_onprem_test).lower(),
        "user": value("username"),
    }
    command = [terraform, "apply", "-input=false", "-no-color", "--auto-approve"]
    command.extend(f"-var={name}={variable_value}" for name, variable_value in variables.items())
    return command


def _terraform_destroy_command(
    terraform: str, case: dict[str, Any], shared: argparse.Namespace
) -> list[str]:
    return [
        terraform,
        "destroy",
        "-input=false",
        "-no-color",
        "-lock-timeout=5m",
        f"-var=region={shared.region}",
        f"-var=ami={case.get('ami', '')}",
        "--auto-approve",
    ]


def _destroy_with_retries(
    terraform: str,
    case: dict[str, Any],
    shared: argparse.Namespace,
    terraform_directory: Path,
    log_file: Any,
    environment: dict[str, str],
) -> tuple[int, bool, float]:
    status = 1
    timed_out = False
    duration_seconds = 0.0
    for attempt in range(1, 3):
        print(f"Terraform destroy attempt {attempt}", file=log_file, flush=True)
        status, timed_out, attempt_seconds = _run_command(
            _terraform_destroy_command(terraform, case, shared),
            terraform_directory,
            log_file,
            shared.destroy_timeout_seconds,
            environment,
        )
        duration_seconds += attempt_seconds
        if status == 0:
            return status, timed_out, duration_seconds
        if attempt < 2 and not CANCELLED.wait(5):
            continue
        break
    return status, timed_out, duration_seconds


def _prepare_case(case_root: Path, source_directory: Path) -> None:
    if case_root.exists():
        shutil.rmtree(case_root)
    case_root.mkdir(parents=True)
    shutil.copytree(source_directory, case_root / "test-repo", symlinks=True)


def _run_case(
    index: int,
    case: dict[str, Any],
    shared: argparse.Namespace,
    cleanup_only: bool,
) -> CaseResult:
    started_at = time.monotonic()
    name = _case_name(case, index)
    result = CaseResult(name=name, wip=bool(case.get("wip", False)))
    case_root = shared.work_root / f"case-{index}"
    log_prefix = "cleanup-" if cleanup_only else ""
    log_path = shared.work_root / "logs" / f"{log_prefix}case-{index}.log"
    log_path.parent.mkdir(parents=True, exist_ok=True)

    if not cleanup_only:
        if index and CANCELLED.wait(shared.launch_delay_seconds * index):
            result.cancelled = True
            result.total_seconds = time.monotonic() - started_at
            return result
        _prepare_case(case_root, shared.source_dir)
    elif not case_root.exists():
        return result

    terraform_directory = _case_working_directory(
        case_root, case, shared.default_terraform_dir
    )
    if not terraform_directory.is_dir():
        raise FileNotFoundError(f"Terraform directory does not exist: {terraform_directory}")

    environment = os.environ.copy()
    terraform = os.environ.get("TERRAFORM_BIN", "terraform")
    with log_path.open("a", encoding="utf-8") as log_file:
        print(f"Running {name} in {terraform_directory}", file=log_file, flush=True)

        if cleanup_only:
            (
                result.init_status,
                result.timed_out,
                result.init_seconds,
            ) = _run_command(
                [terraform, "init", "-input=false", "-no-color"],
                terraform_directory,
                log_file,
                shared.init_timeout_seconds,
                environment,
            )
            if result.init_status == 0:
                (
                    result.destroy_status,
                    destroy_timed_out,
                    result.destroy_seconds,
                ) = _destroy_with_retries(
                    terraform,
                    case,
                    shared,
                    terraform_directory,
                    log_file,
                    environment,
                )
                result.timed_out = result.timed_out or destroy_timed_out
            else:
                result.destroy_status = result.init_status
            result.total_seconds = time.monotonic() - started_at
            return result

        result.init_status, result.timed_out, result.init_seconds = _run_command(
            [terraform, "init", "-input=false", "-no-color"],
            terraform_directory,
            log_file,
            shared.init_timeout_seconds,
            environment,
        )
        if result.init_status == 0:
            result.apply_status, apply_timed_out, result.apply_seconds = _run_command(
                _terraform_apply_command(terraform, case, shared),
                terraform_directory,
                log_file,
                shared.apply_timeout_seconds,
                environment,
            )
            result.timed_out = result.timed_out or apply_timed_out
            (
                result.destroy_status,
                destroy_timed_out,
                result.destroy_seconds,
            ) = _destroy_with_retries(
                terraform,
                case,
                shared,
                terraform_directory,
                log_file,
                environment,
            )
            result.timed_out = result.timed_out or destroy_timed_out
    result.total_seconds = time.monotonic() - started_at
    return result


def _run_case_safely(
    index: int,
    case: dict[str, Any],
    shared: argparse.Namespace,
    cleanup_only: bool,
) -> CaseResult:
    try:
        return _run_case(index, case, shared, cleanup_only)
    except Exception:
        name = _case_name(case, index)
        log_prefix = "cleanup-" if cleanup_only else ""
        log_path = shared.work_root / "logs" / f"{log_prefix}case-{index}.log"
        log_path.parent.mkdir(parents=True, exist_ok=True)
        with log_path.open("a", encoding="utf-8") as log_file:
            traceback.print_exc(file=log_file)
        result = CaseResult(name=name, wip=bool(case.get("wip", False)))
        if cleanup_only:
            result.destroy_status = 1
        else:
            result.init_status = 1
        return result


def _print_logs(work_root: Path, case_count: int, cleanup_only: bool) -> None:
    log_prefix = "cleanup-" if cleanup_only else ""
    for index in range(case_count):
        log_path = work_root / "logs" / f"{log_prefix}case-{index}.log"
        if not log_path.exists():
            continue
        print(f"::group::Integration test case {index + 1}")
        print(log_path.read_text(encoding="utf-8", errors="replace"), end="")
        print("::endgroup::")


def _write_summary(results: list[CaseResult]) -> None:
    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if not summary_path:
        return
    with Path(summary_path).open("a", encoding="utf-8") as summary:
        summary.write("### Paired EC2 integration tests\n\n")
        summary.write("| Test | Result | WIP | Cleanup | Duration |\n")
        summary.write("|---|---|---:|---|---:|\n")
        for result in results:
            if result.cancelled:
                outcome = "cancelled"
            elif result.execution_failed:
                outcome = "failed"
            else:
                outcome = "passed"
            cleanup = "failed" if result.cleanup_failed else "passed"
            summary.write(
                f"| `{result.name}` | {outcome} | "
                f"{'yes' if result.wip else 'no'} | {cleanup} | "
                f"{result.total_seconds / 60:.1f}m |\n"
            )


def _report_results(
    results: list[CaseResult], results_path: Path, cleanup_only: bool
) -> int:
    results_path.parent.mkdir(parents=True, exist_ok=True)
    results_path.write_text(
        json.dumps([asdict(result) for result in results], indent=2) + "\n",
        encoding="utf-8",
    )
    if not cleanup_only:
        _write_summary(results)

    failed = False
    for result in results:
        if result.cancelled:
            action = "cleanup" if cleanup_only else "integration test"
            print(f"::error::{result.name}: {action} was cancelled")
            failed = True
        elif result.cleanup_failed:
            print(f"::error::{result.name}: Terraform cleanup failed")
            failed = True
        elif cleanup_only:
            print(f"::notice::{result.name}: fallback Terraform cleanup passed")
        elif result.execution_failed and result.wip:
            print(f"::warning::{result.name}: WIP test failed and was overruled")
        elif result.execution_failed:
            print(f"::error::{result.name}: integration test failed")
            failed = True
        elif result.wip:
            print(f"::notice::{result.name}: WIP test passed")
    return 1 if failed else 0


def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run all cases in one paired EC2 integration-test matrix entry."
    )
    parser.add_argument("--pair-json", required=True)
    parser.add_argument("--source-dir", type=Path, required=True)
    parser.add_argument("--work-root", type=Path, required=True)
    parser.add_argument("--default-terraform-dir", required=True)
    parser.add_argument("--mode", choices=("run", "cleanup"), default="run")
    parser.add_argument("--build-id", default=os.environ.get("INPUT_BUILD_ID", ""))
    parser.add_argument("--test-repo-url", default=os.environ.get("INPUT_TEST_REPO_URL", ""))
    parser.add_argument(
        "--test-repo-branch", default=os.environ.get("INPUT_TEST_REPO_BRANCH", "")
    )
    parser.add_argument("--localstack-host", default=os.environ.get("INPUT_LOCALSTACK_HOST", ""))
    parser.add_argument("--plugins", default=os.environ.get("INPUT_PLUGINS", ""))
    parser.add_argument("--region", default=os.environ.get("INPUT_REGION", ""))
    parser.add_argument("--s3-bucket", default=os.environ.get("INPUT_S3_BUCKET", ""))
    parser.add_argument(
        "--is-selinux-test",
        action="store_true",
        default=os.environ.get("INPUT_IS_SELINUX_TEST", "false").lower() == "true",
    )
    parser.add_argument(
        "--is-onprem-test",
        action="store_true",
        default=os.environ.get("INPUT_IS_ONPREM_TEST", "false").lower() == "true",
    )
    parser.add_argument(
        "--launch-delay-seconds",
        type=int,
        default=int(os.environ.get("PAIR_LAUNCH_DELAY_SECONDS", "5")),
    )
    parser.add_argument(
        "--init-timeout-seconds",
        type=int,
        default=int(os.environ.get("TERRAFORM_INIT_TIMEOUT_SECONDS", "600")),
    )
    parser.add_argument(
        "--apply-timeout-seconds",
        type=int,
        default=int(os.environ.get("TERRAFORM_APPLY_TIMEOUT_SECONDS", "3600")),
    )
    parser.add_argument(
        "--destroy-timeout-seconds",
        type=int,
        default=int(os.environ.get("TERRAFORM_DESTROY_TIMEOUT_SECONDS", "480")),
    )
    args = parser.parse_args()
    args.source_dir = args.source_dir.resolve()
    args.work_root = args.work_root.resolve()
    return args


def main() -> int:
    args = _parse_args()
    entry = json.loads(args.pair_json)
    if not isinstance(entry, dict):
        raise ValueError("matrix entry must be a JSON object")
    pair = normalize_matrix_entry(entry)
    cases = pair["cases"]

    signal.signal(signal.SIGINT, _handle_signal)
    signal.signal(signal.SIGTERM, _handle_signal)

    args.work_root.mkdir(parents=True, exist_ok=True)
    cleanup_only = args.mode == "cleanup"
    pair_started_at = time.monotonic()
    with concurrent.futures.ThreadPoolExecutor(max_workers=len(cases)) as executor:
        futures = [
            executor.submit(_run_case_safely, index, case, args, cleanup_only)
            for index, case in enumerate(cases)
        ]
        pending = set(futures)
        while pending:
            _, pending = concurrent.futures.wait(
                pending,
                timeout=60,
                return_when=concurrent.futures.FIRST_COMPLETED,
            )
            if pending:
                print(f"{len(pending)} paired integration case(s) still running", flush=True)
        results = [future.result() for future in futures]

    print(
        f"Paired case wall time: {(time.monotonic() - pair_started_at) / 60:.1f} minutes",
        flush=True,
    )
    _print_logs(args.work_root, len(cases), cleanup_only)
    return _report_results(
        results,
        args.work_root / f"{args.mode}-results.json",
        cleanup_only,
    )


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
