---
name: docs-drift-review
description: Analyze repository documentation for implementation drift, stale examples, missing coverage, spelling/clarity issues, and reorganization opportunities; produce a prioritized report with exact file references and proposed fixes.
version: 1.0.0
---

# Docs Drift Review

## Purpose

Use this skill when the user asks to:

- compare docs with the implementation
- find stale or incorrect docs
- review README, guides, API docs, design docs, comments, or onboarding docs
- suggest documentation improvements
- identify spelling, grammar, naming consistency, or reorganization issues
- prepare a documentation cleanup plan
- verify whether a feature or interface changed without corresponding docs updates

## Outcomes

Produce a report that:

1. Identifies implementation drift between docs and code.
2. Distinguishes confirmed issues from plausible-but-unverified concerns.
3. Suggests precise improvements to wording, examples, spelling, structure, and navigation.
4. Prioritizes findings by user impact.
5. Recommends the smallest sensible patch set.

## Non-goals

Do not:

- invent behavior not supported by code or tests
- rewrite all docs when a targeted patch is enough
- flag style preferences as defects unless they reduce clarity or conflict with repo conventions
- claim drift based only on naming differences without checking surrounding context
- modify files unless the user explicitly asks for edits

## Inputs to inspect

Prefer to inspect, in this order:

1. `AGENTS.md` and nearby agent instructions
2. top-level `README*`
3. docs directories such as `docs/`, `documentation/`, `guides/`, `examples/`
4. API specs and schemas such as OpenAPI, JSON Schema, protobuf, CRDs, GraphQL schema
5. source code implementing the documented behavior
6. tests covering the documented behavior
7. config files, CLI help text, and sample manifests
8. changelogs, migration guides, release notes

## Core workflow

### Step 1: Build a docs map

Create a compact inventory:

- major doc files and their purpose
- feature areas covered
- likely source-of-truth files for each area
- stale-looking sections such as versioned commands, flags, env vars, API paths, screenshots, copied outputs, or step-by-step flows

### Step 2: Build an implementation map

Identify:

- entrypoints
- public interfaces
- commands, flags, config keys, env vars
- API routes, request/response shapes
- feature gates, defaults, constraints
- examples and fixtures
- relevant tests

Prefer tests and schemas over comments when determining current behavior.

### Step 3: Compare docs against implementation

Check for:

- renamed or removed commands, flags, env vars, config fields, APIs
- changed defaults
- changed prerequisites or setup steps
- outdated examples or sample outputs
- undocumented new behavior
- docs claiming support that code no longer provides
- docs omitting constraints, edge cases, or failure modes that matter in practice

For each suspected drift item:

- find exact evidence in docs
- find exact evidence in code/tests/specs
- decide: confirmed drift, likely drift, or insufficient evidence

### Step 4: Review doc quality

Inspect for:

- spelling and grammar errors
- inconsistent naming of products, features, commands, files, or concepts
- ambiguous wording
- duplicate content across files
- weak information architecture
- sections in the wrong place
- poor scannability
- examples without explanation
- explanation without runnable examples
- missing cross-links
- outdated references to file paths or repo layout

### Step 5: Produce a prioritized report

Organize findings into:

- Critical correctness issues
- Important missing or stale content
- Clarity and usability improvements
- Copyediting issues
- Reorganization proposals

For each finding include:

- severity: critical / high / medium / low
- confidence: confirmed / likely / uncertain
- docs file(s)
- implementation file(s)
- concise explanation
- recommended fix
- optional proposed replacement text

## Severity guide

Use:

- **critical** for setup-breaking, security-relevant, migration-breaking, or API-breaking documentation errors
- **high** for materially misleading docs that waste time or cause wrong usage
- **medium** for missing context, omissions, stale examples, or structural issues that impair success
- **low** for spelling, grammar, naming consistency, and polish

## Evidence rules

- Prefer direct evidence from code, tests, schemas, generated help, or examples.
- If behavior is ambiguous, say so explicitly.
- Do not infer runtime behavior solely from type names or comments.
- If tests contradict prose docs, treat that as a drift signal and note the contradiction.
- If docs contradict code but code appears accidental or buggy, do not decide policy; report the mismatch neutrally.

## Output format

Use this structure:

### Summary

- brief statement of overall docs health
- count of findings by severity
- top 3 issues

### Findings

For each finding:

- ID
- Severity
- Confidence
- Docs evidence
- Implementation evidence
- Why this matters
- Recommended fix
- Proposed text (only when helpful)

### Coverage gaps

List feature areas implemented but not documented.

### Reorganization suggestions

Suggest merges, splits, moves, or index pages.

### Quick wins

List the smallest high-value doc fixes.

### Optional patch plan

If the user asked for edits, propose a safe file-by-file sequence.

## Review heuristics

Look especially for these drift patterns:

- README says one command, CLI help exposes another
- docs list flags not present in parser definitions
- docs omit required env vars or credentials now enforced by code
- examples use old API paths or field names
- docs describe defaults that changed in config code
- migration guides no longer match actual upgrade path
- docs refer to directories or files moved in the repo
- generated artifacts or CRDs changed but prose docs did not
- screenshots or output snippets no longer match current UX or logs

## Spelling and style heuristics

Flag:

- repeated words
- obvious spelling mistakes
- inconsistent capitalization of product names
- inconsistent code font for commands, filenames, flags, env vars
- headings that do not match the section content
- paragraphs that should be converted into steps, tables, or bullet lists
- long pages that need a TOC or subsection split

Do not overcorrect for house style unless a local convention is clear.

## Reorganization heuristics

Suggest reorganization when:

- the same concept is explained in 3 or more places
- onboarding and reference material are mixed together
- migration guidance is buried in release notes
- examples are detached from the feature they demonstrate
- the top-level README is doing too much
- there is no single source of truth for configuration or API reference

## When asked to edit

If the user wants changes:

1. edit the highest-severity correctness issues first
2. then fix stale examples and missing prerequisites
3. then improve naming consistency and structure
4. keep diffs minimal and reviewable
5. preserve established terminology unless the repo clearly moved on

## Suggested prompts

Examples of requests this skill should handle:

- "Review the docs for implementation drift."
- "Compare README and docs/ against the current CLI and config."
- "Find stale examples and propose fixes."
- "Audit API docs versus the server implementation."
- "Suggest doc reorganization and copyediting improvements."

## Completion criteria

The task is complete when:

- the main doc surfaces were checked against the current implementation
- findings are evidence-based and prioritized
- the report clearly separates confirmed issues from guesses
- suggested fixes are concrete enough to implement
