---
name: eden-microservice
description: Route repository engineering tasks to focused workflows. Use when working on Eden Microservice.
---

# Eden Microservice Router

Read `AGENTS.md`, applicable specifications, affected code/tests, and matching pitfalls. Preserve unrelated changes.

| Request | Use |
| --- | --- |
| Failure, regression, incorrect output, or slowness | `eden-microservice-diagnose` |
| Behavior change or bug fix | `eden-microservice-tdd` |
| Review or safety assessment | `eden-microservice-code-review` |
| New concept, ownership conflict, or ambiguous vocabulary | `eden-microservice-domain-modeling` |

Use the smallest matching skill. Make routine in-scope decisions without a clarification loop; ask only when a missing choice changes behavior or authority.
