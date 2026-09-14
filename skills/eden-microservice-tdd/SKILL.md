---
name: eden-microservice-tdd
description: Implement observable behavior changes and fixes test-first, excluding documentation-only edits.
---

# eden-microservice-tdd

Follow root `AGENTS.md`.

1. Choose the highest existing public test boundary; avoid a new abstraction when one exists.
2. Write and run a focused failing behavior test, then make the smallest change to pass it.
3. Refactor only while green; run required verification. Test observable outcomes, not private calls.
4. Align contracts, API, types, routes, storage, and UI for cross-layer changes.
