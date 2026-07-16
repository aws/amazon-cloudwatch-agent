#!/usr/bin/env python3
# Parse `aws s3 ls --recursive` output (piped through awk '{print $3, $4}')
# and produce a DynamoDB item JSON with binary sizes.
#
# Usage: aws s3 ls ... | awk '{print $3, $4}' | python3 parse-s3-binaries.py
#
# Required env: S3_PREFIX, COMMIT_HASH, COMMIT_DATE
# Optional env: TAG

import json
import os
import sys

prefix = os.environ["S3_PREFIX"]
commit_hash = os.environ["COMMIT_HASH"]
commit_date = os.environ["COMMIT_DATE"]
tag = os.environ.get("TAG", "")

SKIP_EXT = (".sig", ".jar", ".rpm", ".deb", ".pkg", ".msi", ".tar.gz", ".gz", ".zip")
SKIP_NAMES = ("CWAGENT_VERSION", "buildMSI.zip")

binaries = {}
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    parts = line.split(" ", 1)
    if len(parts) != 2:
        continue
    try:
        size, key = int(parts[0]), parts[1]
    except ValueError:
        continue
    rel_path = key[len(prefix):] if key.startswith(prefix) else key
    segments = rel_path.split("/")
    if len(segments) != 2:
        continue
    filename = segments[1]
    if any(filename.endswith(ext) for ext in SKIP_EXT):
        continue
    if filename in SKIP_NAMES:
        continue
    binaries[rel_path] = {"N": str(size)}

if not binaries:
    sys.exit(1)

record_type = "release" if tag else "commit"
item = {
    "CommitHash": {"S": commit_hash},
    "CommitDate": {"N": commit_date},
    "Branch": {"S": "main"},
    "RecordType": {"S": record_type},
    "Binaries": {"M": binaries},
}
if tag:
    item["Tag"] = {"S": tag}

print(json.dumps(item))
