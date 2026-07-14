# Eden Microservice Engineering Plugin

Optional Codex enhancement. The source of truth remains `AGENTS.md`, `specs/`, and `skills/eden-microservice/SKILL.md`; Claude Code follows the same files without installing this plugin.

The bundle provides five skills, a local MCP server for task context and confirmed pitfall lookup, a non-mutating write guard, and `tooling/lsp.json`. Verify or install language servers with `scripts/check-language-tools.ps1 -Install`.

Focused skills: `eden-microservice-diagnose`, `eden-microservice-tdd`, `eden-microservice-code-review`, and `eden-microservice-domain-modeling`.
