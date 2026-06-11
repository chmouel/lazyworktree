#!/usr/bin/env bash
set -euo pipefail

find . \
  -type f \
  \( -iname "*.go" -o -iname "*.py" -o -iname "*.ts" -o -iname "*.tsx" -o -iname "*.js" -o -iname "*.jsx" -o -iname "*.java" -o -iname "*.rs" -o -iname "*.yaml" -o -iname "*.yml" -o -iname "*.json" -o -iname "*.proto" \) |
  sort
