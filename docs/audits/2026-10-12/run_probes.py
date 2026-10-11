"""Reproduce audit observations without modifying production or test files.

PASS means the reported defect/limitation was observed. These are observation
probes, not acceptance tests proving a fix. A subsequent fix can make them fail.
All providers and credentials are fake; no live vendor requests are needed.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile

source = Path(__file__).resolve().parent
root = source.parents[2]
packages = ["provider", "cloudsync", "app", "agent", "e2e"]
with tempfile.TemporaryDirectory(prefix="memo-audit-overlay-") as temp_dir:
    temp = Path(temp_dir)
    replacements = {}
    for package in packages:
        virtual = root / "internal" / package / "audit_observation_test.go"
        if virtual.exists():
            raise SystemExit(f"Refusing to shadow existing file: {virtual}")
        real = temp / f"{package}_probe_test.go"
        real.write_bytes((source / f"{package}_probe_test.go.txt").read_bytes())
        replacements[str(virtual)] = str(real)
    overlay = temp / "overlay.json"
    overlay.write_text(json.dumps({"Replace": replacements}))
    env = dict(os.environ, CGO_ENABLED="1", MEMO_DATA_DIR=str(temp / "data"))
    subprocess.run(
        ["go", "test", "-tags", "sqlite_fts5", "-overlay", str(overlay),
         *[f"./internal/{package}" for package in packages],
         "-run", "^TestAudit", "-count=1", "-race", "-v"],
        cwd=root, env=env, check=True,
    )
