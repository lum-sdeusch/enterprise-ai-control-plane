#!/usr/bin/env python3
"""Validate every tool, agent and team manifest against its JSON schema and cross-check
references between them. Exit 1 on any error. Warnings are printed and do not fail the run;
the tool gate (scripts/tool-gate.sh) turns the placeholder warnings into errors at merge time.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from eacp_manifests import load_all, report  # noqa: E402

if __name__ == "__main__":
    sys.exit(report(load_all()))
