#!/bin/bash
# Record binary sizes to DynamoDB after a main merge.
# Reads sizes from S3 (binaries already uploaded by BuildAndUpload job).
#
# Required env: S3_BUCKET, COMMIT_HASH, COMMIT_DATE, TAG (optional)

set -euo pipefail

TABLE_NAME="CWABinarySizes"
S3_BUCKET="${S3_BUCKET:?S3_BUCKET is required}"
COMMIT_HASH="${COMMIT_HASH:?COMMIT_HASH is required}"
COMMIT_DATE="${COMMIT_DATE:?COMMIT_DATE is required}"
TAG="${TAG:-}"
REGION="us-west-2"

S3_PREFIX="integration-test/binary/${COMMIT_HASH}/"

echo "Collecting binary sizes from S3 (${COMMIT_HASH:0:12})..."

ITEM_JSON=$(
     aws s3 ls "s3://${S3_BUCKET}/${S3_PREFIX}" --recursive --region "$REGION" |
          awk '{print $3, $4}' |
          python3 - "$S3_PREFIX" "$COMMIT_HASH" "$COMMIT_DATE" "$TAG" <<'PYEOF'
import sys, json

prefix = sys.argv[1]
commit_hash = sys.argv[2]
commit_date = sys.argv[3]
tag = sys.argv[4]

skip_ext = ('.sig', '.jar', '.rpm', '.deb', '.pkg', '.msi', '.tar.gz', '.gz', '.zip')
skip_names = ('CWAGENT_VERSION', 'buildMSI.zip')

binaries = {}
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    parts = line.split(' ', 1)
    if len(parts) != 2:
        continue
    size, key = int(parts[0]), parts[1]
    rel_path = key[len(prefix):] if key.startswith(prefix) else key
    segments = rel_path.split('/')
    if len(segments) != 2:
        continue
    filename = segments[1]
    if any(filename.endswith(ext) for ext in skip_ext):
        continue
    if filename in skip_names:
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
PYEOF
) || {
     echo "No binaries found in S3, skipping."
     exit 0
}

RECORD_TYPE=$(echo "$ITEM_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['RecordType']['S'])")

if [[ "$RECORD_TYPE" == "release" ]]; then
     aws dynamodb put-item \
          --table-name "$TABLE_NAME" \
          --item "$ITEM_JSON" \
          --region "$REGION"
else
     err=$(aws dynamodb put-item \
          --table-name "$TABLE_NAME" \
          --item "$ITEM_JSON" \
          --condition-expression "attribute_not_exists(CommitHash) OR RecordType <> :release" \
          --expression-attribute-values '{":release": {"S": "release"}}' \
          --region "$REGION" 2>&1) || {
          if echo "$err" | grep -q "ConditionalCheckFailedException"; then
               echo "Skipped (existing release entry)"
          else
               echo "$err" >&2
               exit 1
          fi
     }
fi

echo "Recorded binary sizes for ${COMMIT_HASH:0:12} (type=${RECORD_TYPE}, tag=${TAG:-none})"
