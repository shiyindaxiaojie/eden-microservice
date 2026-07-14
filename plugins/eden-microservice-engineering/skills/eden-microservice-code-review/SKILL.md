---
name: eden-microservice-code-review
description: Review a diff or change for contract fidelity and engineering risk. Use when working on Eden Microservice.
---

# Eden Microservice Code Review

Review is read-only unless fixes are requested.

1. Contract pass: check the governing specification, identity, authorization, configuration, and cross-layer alignment.
2. Engineering pass: inspect scoped diff and tests for regressions, boundary violations, unsafe defaults, incomplete errors, and needless complexity.
3. Report only actionable findings with file/line evidence and a concrete failure mode.
4. State remaining verification gaps instead of inventing findings.
