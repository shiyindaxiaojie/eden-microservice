---
name: eden-microservice-diagnose
description: Diagnose failures and performance problems before authorized fixes.
---

# eden-microservice-diagnose

Follow root `AGENTS.md`; diagnosis is read-only unless a fix is requested.

1. Build the cheapest stable test/API/CLI/browser/trace reproduction before source edits.
2. Reduce to one scenario with expected/actual behavior; test one falsifiable hypothesis at a time.
3. For an authorized fix, correct the verified cause and retain a regression test at the
   highest existing public boundary. Report reproduction, cause, boundary, and fresh verification.
