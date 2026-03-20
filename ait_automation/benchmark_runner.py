from __future__ import annotations

import shlex
import subprocess
import time
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, List, Sequence


@dataclass
class JobConfig:
    model: str
    concurrency: int
    count: int
    protocol: str | None = None
    base_url: str | None = None
    api_key: str | None = None
    stream: bool | None = None
    thinking: bool | None = None
    timeout: int | None = None
    prompt: str | None = None
    prompt_file: str | None = None
    prompt_length: int | None = None
    interval_report: int | None = None
    max_tokens: int | None = None
    temperature: float | None = None
    top_p: float | None = None
    top_k: int | None = None


@dataclass
class RunResult:
    return_code: int
    elapsed_seconds: float
    new_report_files: List[Path]
    command: List[str]


class AITBenchmarkRunner:
    def __init__(
        self,
        command: Sequence[str] | str,
        workdir: Path,
        env: Dict[str, str] | None = None,
        report_pattern: str = "ait-report-*.json",
        report_wait_seconds: int = 5,
        heartbeat_seconds: int = 15,
    ) -> None:
        if isinstance(command, str):
            self.command = shlex.split(command)
        else:
            self.command = list(command)
        self.workdir = workdir.resolve()
        self.env = dict(env or {})
        self.report_pattern = report_pattern
        self.report_wait_seconds = report_wait_seconds
        self.heartbeat_seconds = heartbeat_seconds

    def _snapshot_reports(self) -> set[Path]:
        return {p.resolve() for p in self.workdir.glob(self.report_pattern)}

    @staticmethod
    def _flag(name: str, value: object) -> str:
        if isinstance(value, bool):
            return f"--{name}={'true' if value else 'false'}"
        return f"--{name}={value}"

    def _build_job_args(self, job: JobConfig) -> List[str]:
        args: List[str] = []
        args.append(self._flag("model", job.model))
        args.append(self._flag("concurrency", job.concurrency))
        args.append(self._flag("count", job.count))
        args.append(self._flag("report", True))

        optional_map = {
            "protocol": job.protocol,
            "baseUrl": job.base_url,
            "apiKey": job.api_key,
            "stream": job.stream,
            "thinking": job.thinking,
            "timeout": job.timeout,
            "prompt": job.prompt,
            "prompt-file": job.prompt_file,
            "prompt-length": job.prompt_length,
            "interval-report": job.interval_report,
            "max-tokens": job.max_tokens,
            "temperature": job.temperature,
            "top-p": job.top_p,
            "top-k": job.top_k,
        }
        for name, value in optional_map.items():
            if value is None:
                continue
            args.append(self._flag(name, value))
        return args

    def run_job(self, job: JobConfig, index: int, total: int) -> RunResult:
        before = self._snapshot_reports()
        cmd = self.command + self._build_job_args(job)

        print(
            f"[RUN {index}/{total}] start: model={job.model}, concurrency={job.concurrency}, "
            f"count={job.count}, stream={job.stream}, thinking={job.thinking}"
        )
        print(f"[RUN {index}/{total}] command: {' '.join(cmd)}")

        started = time.time()
        proc = subprocess.Popen(
            cmd,
            cwd=str(self.workdir),
            env={**dict(os.environ), **self.env},
        )

        next_beat = started + self.heartbeat_seconds
        while proc.poll() is None:
            now = time.time()
            if now >= next_beat:
                elapsed = int(now - started)
                print(
                    f"[RUN {index}/{total}] running... elapsed={elapsed}s "
                    f"(model={job.model}, concurrency={job.concurrency}, count={job.count})"
                )
                next_beat = now + self.heartbeat_seconds
            time.sleep(1)

        elapsed = time.time() - started
        return_code = proc.returncode or 0

        after = self._snapshot_reports()
        new_files = sorted(after - before)
        if not new_files:
            deadline = time.time() + self.report_wait_seconds
            while time.time() < deadline:
                time.sleep(1)
                after = self._snapshot_reports()
                new_files = sorted(after - before)
                if new_files:
                    break

        print(
            f"[RUN {index}/{total}] done: rc={return_code}, elapsed={elapsed:.1f}s, "
            f"new_json_reports={len(new_files)}"
        )
        for file_path in new_files:
            print(f"[RUN {index}/{total}] report: {file_path}")

        return RunResult(
            return_code=return_code,
            elapsed_seconds=elapsed,
            new_report_files=new_files,
            command=cmd,
        )
