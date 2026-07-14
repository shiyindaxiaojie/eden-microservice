---
name: eden-microservice-diagnose
description: Diagnose failures, regressions, incorrect output, or performance problems. Use when working on Eden Microservice.
---

# Eden Microservice Diagnose

1. Read the governing specification, affected code/tests, relevant `CONTEXT.md` terms, and pitfalls.
2. Build the cheapest stable pass/fail signal before source changes: focused test, API/CLI reproduction, browser assertion, trace replay, or minimal harness.
3. Reduce to one scenario; record expected and actual behavior.
4. Test one falsifiable hypothesis at a time.
5. Fix the verified cause and retain a regression test at the highest existing public seam.
6. Report reproduction, cause, seam, and fresh verification. Keep diagnosis read-only unless a fix is requested.
