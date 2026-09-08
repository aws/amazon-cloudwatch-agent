#!/bin/bash
# Record binary sizes to DynamoDB after a main merge.
# Reads sizes from S3 (binaries already uploaded by BuildAndUpload job).
#
# Required env: S3_BUCKET, COMMIT_HASH, COMMIT_DATE, TAG (optional)

set -euo pipefail

TABLE_NAME="CWABinarySizes"
S3_BUCKET="${S3_BUCKET:?S3_BUCKET is required}"
export COMMIT_HASH="${COMMIT_HASH:?COMMIT_HASH is required}"
export COMMIT_DATE="${COMMIT_DATE:?COMMIT_DATE is required}"
export TAG="${TAG:-}"
REGION="us-west-2"

export S3_PREFIX="integration-test/binary/${COMMIT_HASH}/"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Collecting binary sizes from S3 (${COMMIT_HASH:0:12})..."

S3_LISTING=$(aws s3 ls "s3://${S3_BUCKET}/${S3_PREFIX}" --recursive --region "$REGION" || true)
if [[ -z "$S3_LISTING" ]]; then
     echo "No binaries found in S3, skipping."
     exit 0
fi

ITEM_JSON=$(
     echo "$S3_LISTING" | awk '{print $3, $4}' | python3 "$SCRIPT_DIR/parse-s3-binaries.py"
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
