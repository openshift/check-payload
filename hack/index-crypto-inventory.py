#!/usr/bin/env python3
"""Build a central artifact index from schema-validated crypto reports."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import sys

from jsonschema import Draft202012Validator

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--output", type=Path)
parser.add_argument("--artifact-root", type=Path, default=Path("."))
parser.add_argument("--expected-report", action="append", type=Path, default=[])
parser.add_argument("reports", nargs="*", type=Path)
args = parser.parse_args()
if not args.reports and not args.expected_report:
    parser.error("provide reports or expected report paths")
schema = json.loads((Path(__file__).resolve().parents[1] /
                     "internal/inventory/report.schema.json").read_text())
Draft202012Validator.check_schema(schema)
validator = Draft202012Validator(schema)
entries = []
for report in args.reports:
    data = report.read_bytes()
    doc = json.loads(data)
    validator.validate(doc)
    artifact_path = report.resolve().relative_to(args.artifact_root.resolve())
    entries.append({
        "report": artifact_path.as_posix(),
        "sha256": hashlib.sha256(data).hexdigest(),
        "component": doc.get("component", ""),
        "mode": doc["mode"],
        "scannerVersion": doc["scannerVersion"],
        "policyVersion": doc["policyVersion"],
        "complete": all(a["coverage"] == "analyzed" for a in doc["artifacts"]),
        "artifacts": [{
            "path": a["path"], "image": a.get("image", ""),
            "sourceCommit": a.get("sourceCommit", ""),
            "coverage": a["coverage"],
            "classificationCounts": dict(sorted(Counter(
                f["classification"] for f in a["findings"]).items())),
        } for a in doc["artifacts"]],
    })
present = {report.resolve() for report in args.reports}
missing = sorted({report.resolve().relative_to(args.artifact_root.resolve()).as_posix()
                  for report in args.expected_report if report.resolve() not in present})
index = {"schemaVersion": "crypto-inventory-index/v1",
         "complete": bool(entries) and not missing and all(e["complete"] for e in entries),
         "missingReports": missing,
         "reports": sorted(entries, key=lambda e: e["report"])}
text = json.dumps(index, indent=2) + "\n"
if args.output:
    args.output.write_text(text)
else:
    sys.stdout.write(text)
