---
name: eden-microservice-domain-modeling
description: Resolve new or ambiguous terms, ownership, identity, and durable design decisions.
---

# eden-microservice-domain-modeling

Follow root `AGENTS.md`; read relevant `CONTEXT.md`, contract sections, and code.

1. Define one canonical term, owner, source of truth, identity, lifecycle, and external interactions.
2. Reject ambiguous aliases; preserve API/storage/console vocabulary. Prefer owned behavior and
   explicit public interfaces over cross-module implementation imports.
3. Update `CONTEXT.md` only for confirmed reusable terms and specifications for contract changes.
