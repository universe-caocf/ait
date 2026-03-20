#!/usr/bin/env python3
from __future__ import annotations

import argparse
from pathlib import Path

try:
    from .workflow import run_workflow
except ImportError:
    from workflow import run_workflow


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="AIT automation workflow entrypoint")
    parser.add_argument(
        "--config",
        default="ait_automation/config.example.yaml",
        help="Path to workflow YAML config",
    )
    parser.add_argument("--dry-run", action="store_true", help="Print planned runs and exit")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    return run_workflow(config_path=Path(args.config), dry_run=args.dry_run)


if __name__ == "__main__":
    raise SystemExit(main())
