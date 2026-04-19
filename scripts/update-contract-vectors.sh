#!/usr/bin/env bash
# Refresh the vendored contract vectors in atlasent/testdata/contract/vectors.
# Run from the repo root.
set -euo pipefail

SDK_REPO="${SDK_REPO:-https://raw.githubusercontent.com/AtlaSent-Systems-Inc/atlasent-sdk}"
REF="${REF:-main}"
DEST="atlasent/testdata/contract/vectors"

mkdir -p "${DEST}"
for f in evaluate.json verify.json errors.json headers.json; do
  echo "Fetching ${f} @ ${REF}"
  curl -fSsL -o "${DEST}/${f}" "${SDK_REPO}/${REF}/contract/vectors/${f}"
done
echo "OK — vectors refreshed at ${REF}"
