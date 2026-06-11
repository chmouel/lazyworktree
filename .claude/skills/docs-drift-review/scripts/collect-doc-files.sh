#!/usr/bin/env bash
set -euo pipefail

find . \
  -type f \
  \( -iname "*.md" -o -iname "*.mdx" -o -iname "*.rst" -o -iname "*.adoc" -o -iname "README*" \) |
  sort
