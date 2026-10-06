#!/usr/bin/env python3
"""Validate inventory artifacts against the versioned report contract."""
import argparse
import json
from pathlib import Path

from jsonschema import Draft202012Validator

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("reports", nargs="+", type=Path)
args = parser.parse_args()
schema = json.loads((Path(__file__).resolve().parents[1] /
                     "internal/inventory/report.schema.json").read_text())
Draft202012Validator.check_schema(schema)
validator = Draft202012Validator(schema)
for report in args.reports:
    validator.validate(json.loads(report.read_text()))
    print(f"Validated {report}")
