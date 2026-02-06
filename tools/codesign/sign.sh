#!/bin/bash
# Wrapper script for codesigning macOS binaries
# This script is called by GoReleaser during the release process

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SIGN_BINARY="${SCRIPT_DIR}/sign"

# Check if the sign binary exists
if [[ ! -x "${SIGN_BINARY}" ]]; then
    echo "Warning: sign binary not found at ${SIGN_BINARY}"
    echo "Skipping code signing. Binary will be unsigned."
    echo "See tools/codesign/README.md for instructions on obtaining the sign binary."
    exit 0
fi

# Get the binary to sign from arguments
if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <binary-path> [output-path]"
    exit 1
fi

BINARY_PATH="$1"
OUTPUT_PATH="${2:-$1}"

# Check if the binary exists
if [[ ! -f "${BINARY_PATH}" ]]; then
    echo "Error: Binary not found at ${BINARY_PATH}"
    exit 1
fi

echo "Signing binary: ${BINARY_PATH}"

# Call the sign binary
"${SIGN_BINARY}" "${BINARY_PATH}"

# If output path is different, move the signed binary
if [[ "${OUTPUT_PATH}" != "${BINARY_PATH}" ]]; then
    mv "${BINARY_PATH}" "${OUTPUT_PATH}"
fi

echo "Successfully signed: ${OUTPUT_PATH}"
