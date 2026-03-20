from __future__ import annotations

import itertools
from pathlib import Path
from typing import Any, Dict, List

try:
    from .benchmark_runner import AITBenchmarkRunner, JobConfig
    from .report_to_excel import DEFAULT_FIELDS, ReportCollector
except ImportError:
    from benchmark_runner import AITBenchmarkRunner, JobConfig
    from report_to_excel import DEFAULT_FIELDS, ReportCollector


def load_yaml_config(path: Path) -> Dict[str, Any]:
    try:
        import yaml
    except Exception as exc:
        raise RuntimeError("PyYAML is required. Install with: pip install pyyaml") from exc

    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise ValueError("Config file root must be a mapping/object")
    return data


def _build_jobs(config: Dict[str, Any]) -> List[JobConfig]:
    jobs_cfg = config.get("jobs")
    if isinstance(jobs_cfg, list) and jobs_cfg:
        jobs: List[JobConfig] = []
        for item in jobs_cfg:
            if not isinstance(item, dict):
                raise ValueError("Each item in jobs must be a mapping")
            jobs.append(
                JobConfig(
                    model=str(item["model"]),
                    concurrency=int(item["concurrency"]),
                    count=int(item["count"]),
                    protocol=item.get("protocol"),
                    base_url=item.get("base_url"),
                    api_key=item.get("api_key"),
                    stream=item.get("stream"),
                    thinking=item.get("thinking"),
                    timeout=item.get("timeout"),
                    prompt=item.get("prompt"),
                    prompt_file=item.get("prompt_file"),
                    prompt_length=item.get("prompt_length"),
                    interval_report=item.get("interval_report"),
                    max_tokens=item.get("max_tokens"),
                    temperature=item.get("temperature"),
                    top_p=item.get("top_p"),
                    top_k=item.get("top_k"),
                )
            )
        return jobs

    matrix = config.get("matrix", {})
    if not isinstance(matrix, dict):
        raise ValueError("matrix must be a mapping when jobs is not provided")

    models = matrix.get("models", [])
    concurrencies = matrix.get("concurrencies", [])
    counts = matrix.get("counts", [])
    if not (models and concurrencies and counts):
        raise ValueError("Provide jobs or matrix.models + matrix.concurrencies + matrix.counts")

    default = config.get("defaults", {})
    if not isinstance(default, dict):
        raise ValueError("defaults must be a mapping")

    jobs = []
    for model, concurrency, count in itertools.product(models, concurrencies, counts):
        jobs.append(
            JobConfig(
                model=str(model),
                concurrency=int(concurrency),
                count=int(count),
                protocol=default.get("protocol"),
                base_url=default.get("base_url"),
                api_key=default.get("api_key"),
                stream=default.get("stream"),
                thinking=default.get("thinking"),
                timeout=default.get("timeout"),
                prompt=default.get("prompt"),
                prompt_file=default.get("prompt_file"),
                prompt_length=default.get("prompt_length"),
                interval_report=default.get("interval_report"),
                max_tokens=default.get("max_tokens"),
                temperature=default.get("temperature"),
                top_p=default.get("top_p"),
                top_k=default.get("top_k"),
            )
        )
    return jobs


def run_workflow(config_path: Path, dry_run: bool = False) -> int:
    config = load_yaml_config(config_path.resolve())

    runner_cfg = config.get("runner", {})
    history_cfg = config.get("history", {})
    if not isinstance(runner_cfg, dict) or not isinstance(history_cfg, dict):
        raise ValueError("runner/history must be mapping")

    command = runner_cfg.get("command", ["go", "run", "./cmd/ait"])
    workdir = Path(runner_cfg.get("workdir", ".")).resolve()
    env = runner_cfg.get("env", {})
    report_pattern = str(runner_cfg.get("report_pattern", "ait-report-*.json"))

    output = Path(history_cfg.get("output", "benchmark_history.csv")).resolve()
    state_file = Path(history_cfg.get("state_file", ".ait_report_ingest_state.json")).resolve()
    fields = history_cfg.get("fields", DEFAULT_FIELDS)

    if not isinstance(env, dict):
        raise ValueError("runner.env must be mapping")
    if not isinstance(fields, list) or not all(isinstance(x, str) for x in fields):
        raise ValueError("history.fields must be a list of strings")

    jobs = _build_jobs(config)
    total = len(jobs)

    print(f"[WORKFLOW] config={config_path.resolve()}")
    print(f"[WORKFLOW] workdir={workdir}")
    print(f"[WORKFLOW] total_jobs={total}")
    print(f"[WORKFLOW] history_csv={output}")

    if dry_run:
        for i, job in enumerate(jobs, 1):
            print(
                f"[DRY-RUN {i}/{total}] model={job.model}, concurrency={job.concurrency}, "
                f"count={job.count}, protocol={job.protocol}, base_url={job.base_url}"
            )
        return 0

    collector = ReportCollector(output_file=output, state_file=state_file, fields=fields)
    runner = AITBenchmarkRunner(
        command=command,
        workdir=workdir,
        env={str(k): str(v) for k, v in env.items()},
        report_pattern=report_pattern,
    )

    failed_runs = 0
    appended_rows = 0
    for i, job in enumerate(jobs, 1):
        result = runner.run_job(job, i, total)
        if result.return_code != 0:
            failed_runs += 1
            print(f"[WORKFLOW] run {i}/{total} failed, rc={result.return_code}")

        if result.new_report_files:
            appended = collector.ingest_report_files(result.new_report_files)
            appended_rows += appended
            print(
                f"[WORKFLOW] run {i}/{total} ingested {appended} row(s), "
                f"history_total_added_this_workflow={appended_rows}"
            )
        else:
            print(
                f"[WORKFLOW] run {i}/{total} produced no new JSON report file. "
                "Nothing ingested."
            )

    print(
        f"[WORKFLOW] completed: total_jobs={total}, failed_runs={failed_runs}, "
        f"rows_added={appended_rows}"
    )
    return 1 if failed_runs > 0 else 0
