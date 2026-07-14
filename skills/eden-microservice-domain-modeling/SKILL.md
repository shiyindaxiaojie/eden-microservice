---
name: eden-microservice-domain-modeling
description: Clarify domain language, ownership, identity, and durable decisions. Use when working on Eden Microservice.
---

# Eden Microservice Domain Modeling

1. Read `CONTEXT.md`, relevant specifications, and affected code.
2. Define one canonical term, owner, source of truth, identity fields, lifecycle, and external interactions.
3. Reject ambiguous aliases and preserve existing API, storage, and console vocabulary.
4. Prefer owned behavior and a clear public interface over cross-module implementation imports.
5. Update `CONTEXT.md` only for confirmed reusable terms; update specifications when a contract changes.
