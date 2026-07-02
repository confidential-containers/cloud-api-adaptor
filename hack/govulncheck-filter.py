#!/usr/bin/env python3
"""Filter for govulncheck --json output.

Reads govulncheck JSON stream from stdin, suppresses vulnerability IDs listed
in the ignore file, and exits non-zero only when non-ignored findings remain.

Environment variables:
  IGNORE_FILE   Path to .govulncheck-ignore.toml (required)
  VERBOSE       Set to "true" to pass through config/progress/osv messages
"""

import json
import os
import sys
import tomllib
from datetime import date

ignore_file = os.environ.get("IGNORE_FILE", "")
verbose = os.environ.get("VERBOSE", "false") == "true"
today = date.today()

ignore_reason: dict[str, str] = {}
ignore_until: dict[str, date] = {}
if ignore_file:
    try:
        with open(ignore_file, "rb") as f:
            data = tomllib.load(f)
    except (OSError, tomllib.TOMLDecodeError) as e:
        print(f"govulncheck-filter: failed to load {ignore_file}: {e}", file=sys.stderr)
        sys.exit(2)
    for entry in data.get("IgnoredVulns", []):
        vuln_id = entry.get("id", "")
        if not vuln_id:
            continue
        ignore_reason[vuln_id] = entry.get("reason", "")
        until = entry.get("ignoreUntil")
        if until:
            ignore_until[vuln_id] = until if isinstance(until, date) else date.fromisoformat(str(until))

announced: set[str] = set()
failed = False

# govulncheck --json is pretty-printed; use a streaming decoder to reassemble objects.
decoder = json.JSONDecoder()
buf = ""

for raw_line in sys.stdin:
    buf += raw_line
    while buf.strip():
        buf = buf.lstrip()
        if not buf:
            break
        try:
            msg, idx = decoder.raw_decode(buf)
            buf = buf[idx:]
        except json.JSONDecodeError:
            break

        if "finding" in msg:
            vuln_id = msg["finding"].get("osv", "")
            trace = msg["finding"].get("trace", [])
            # Skip module/package-only findings: govulncheck text mode only
            # fails on call-reachable findings (those with a "function" frame).
            if not any("function" in frame for frame in trace):
                continue
            if vuln_id in ignore_reason:
                exp = ignore_until.get(vuln_id)
                if exp and exp < today:
                    if vuln_id not in announced:
                        print(f"[WARN] Ignore for {vuln_id} has expired ({exp}), please review")
                        announced.add(vuln_id)
                else:
                    if vuln_id not in announced:
                        print(f"[IGNORED] {vuln_id} \u2014 {ignore_reason[vuln_id]}")
                        announced.add(vuln_id)
            else:
                location = ""
                if trace:
                    last = trace[-1]
                    pos = last.get("position")
                    if pos and pos.get("filename"):
                        location = f" ({pos['filename']}:{pos['line']})"
                print(f"Vulnerability: {vuln_id}{location}")
                failed = True

        elif verbose:
            print(json.dumps(msg))

sys.exit(1 if failed else 0)
