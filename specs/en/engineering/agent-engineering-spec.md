# Agent Engineering Guardrails

## Goal

Provide concise, specification-first workflows for Codex, Claude Code, and compatible agents while reducing repeated errors and irrelevant context.

## Focused workflows

`eden-microservice` routes work; `eden-microservice-diagnose` establishes a stable reproduction; `eden-microservice-tdd` implements behavior at a public seam; `eden-microservice-code-review` separates contract and engineering-risk checks; and `eden-microservice-domain-modeling` resolves terminology, ownership, and identity. Read `CONTEXT.md` only for relevant or ambiguous terminology.

Make routine in-scope decisions without a mandatory clarification loop. Ask only when a missing choice changes behavior or authority.

## Context loading

`AGENTS.md` owns shared rules; skills add task-specific decisions. Reuse unchanged context,
load one matching skill copy, and read relevant contract sections/dependencies in one language.
Indexes locate unknown documents; they are not mandatory reading chains. Search pitfalls by scope.
Documentation-only edits use document checks; retain required behavior/module checks, repeating
successful verification only for changes, failures, or unresolved risk.
