#!/usr/bin/env bash
# Author: Chmouel Boudjnah <chmouel@chmouel.com>
set -eufo pipefail

if [ -z "${GEMINI_API_KEY}" ]; then
  echo "GEMINI_API_KEY is required"
  exit 1
fi

if [[ -z ${TAG:-} ]]; then
  TAG=$(git tag -l | tail -1)
  echo "Using latest tag: ${TAG}"
  read -p "Press enter to continue or Ctrl+C to abort"
fi

tag_commit="$(git rev-list -n 1 "${TAG}^{commit}")"
previous_tag="$(git describe --tags --abbrev=0 --match '*.[0-9]*.[0-9]*' "${tag_commit}^" 2>/dev/null || true)"
model="gemini-3-flash-preview"
range="${TAG}"

if [ -n "${previous_tag}" ]; then
  range="${previous_tag}..${TAG}"
fi

git log --patch --no-color "${range}" >/tmp/release-commits.txt
if [ ! -s /tmp/release-commits.txt ]; then
  echo "- None." >/tmp/release-commits.txt
fi

git log --pretty='%s%n%b' "${range}" | grep -Eo '#[0-9]+' | sort -u | sed 's/^/- /' >/tmp/release-prs.txt || true
if [ ! -s /tmp/release-prs.txt ]; then
  echo "- None." >/tmp/release-prs.txt
fi

git log --pretty='- %s%n%b' "${range}" | grep -Ei 'breaking change|breaking-change|!:' >/tmp/release-breaking.txt || true
if [ ! -s /tmp/release-breaking.txt ]; then
  echo "None" >/tmp/release-breaking.txt
fi

cat >/tmp/release-prompt.txt <<EOF
You are generating GitHub release notes for a software project.

GOAL
Write clear, concise, professional GitHub release notes suitable for a public open-source release.

Changes:
$(cat /tmp/release-commits.txt)

Pull Requests:
$(cat /tmp/release-prs.txt)

Breaking Changes:
$(cat /tmp/release-breaking.txt)

INSTRUCTIONS

- Group changes into logical sections:
  - Features
  - Bug Fixes
  - Performance Improvements
  - Maintenance / Refactoring
  - Dependencies
  - Breaking Changes (only if applicable)
- Convert raw commits into user-facing language.
- Merge duplicate or noisy commits into one clean bullet.
- Mention PR numbers when available.
- Do not invent changes.
- Do not include a Contributors section because GitHub handles are not provided in the input.
- Keep tone neutral and professional.
- Keep it scannable (bullets, short sections).

OUTPUT FORMAT (Markdown)

{{1-2 sentence summary based on Highlights or major changes}}

### Features

- ...

### Bug Fixes

- ...

### Performance Improvements

- ...

### Maintenance

- ...

### Dependencies

- ...

### Breaking Changes

- ...

Output only the final Markdown release notes, with no code fences.
EOF

jq -n --rawfile prompt /tmp/release-prompt.txt '{contents:[{parts:[{text:$prompt}]}]}' >/tmp/gemini-request.json

curl -fsS "https://generativelanguage.googleapis.com/v1beta/models/${model}:generateContent?key=${GEMINI_API_KEY}" \
  -H "Content-Type: application/json" \
  -X POST \
  -d @/tmp/gemini-request.json >/tmp/gemini-response.json

if ! jq -er '.candidates[0].content.parts | map(.text // "") | join("") | select(length > 0 and . != "null")' /tmp/gemini-response.json >/tmp/gh-release.md; then
  echo "Gemini returned invalid release notes"
  cat /tmp/gemini-response.json
  exit 1
fi

gh release edit "${TAG}" --notes-file /tmp/gh-release.md
