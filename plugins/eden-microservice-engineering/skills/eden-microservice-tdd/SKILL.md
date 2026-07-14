---
name: eden-microservice-tdd
description: Implement behavior changes and bug fixes test-first. Use when working on Eden Microservice.
---

# Eden Microservice Test-Driven Development

1. Read the governing specification, implementation/tests, and pitfalls.
2. Choose the highest existing public seam; prefer an existing boundary over a new abstraction.
3. Write and run one focused failing behavior test.
4. Make the smallest production change that turns it green.
5. Refactor only while green, then run required module-level verification.
6. Align API, types, routes, storage, and UI when the behavior crosses layers.
