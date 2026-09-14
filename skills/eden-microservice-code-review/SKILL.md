---
name: eden-microservice-code-review
description: Review diffs and implementations for contract fidelity and engineering risk.
---

# eden-microservice-code-review

Follow root `AGENTS.md`; review is read-only unless fixes are requested.

1. Contract pass: check behavior, identity, authorization, configuration, and cross-layer alignment.
2. Engineering pass: check scoped diff/tests for regressions, boundary violations, unsafe defaults,
   incomplete errors, validation gaps, and needless complexity.
3. Report supported, actionable findings by impact with file/line evidence and a concrete failure
   mode. State verification gaps; do not invent findings or turn style preferences into defects.
