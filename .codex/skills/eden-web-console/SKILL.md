---
name: eden-web-console
description: Use this skill when working on the Vue 3 + TypeScript web console in web/, including routes, views, axios API calls, Element Plus UI, auth redirects, dashboard/service pages, settings pages, and frontend build/debug tasks tied to the Eden backend.
---

# Eden Web Console

Use this skill for frontend work inside `web/`.

## What to inspect first

- Read [`references/frontend-map.md`](./references/frontend-map.md).
- Open the target view in `web/src/views`.
- Check shared API access in `web/src/api` and navigation in `web/src/router/index.ts`.

## Core workflow

1. Start from the target route or page component and map the API calls it needs.
2. Reuse existing console patterns: Vue 3 SFCs, router-based pages, axios wrapper in `web/src/api/index.ts`, and Element Plus components.
3. Preserve auth behavior: JWT token from `localStorage`, `Authorization` header injection, and redirect to `/login` on `401`.
4. Keep copy and labels aligned with the existing console style unless the task explicitly requests a redesign.
5. If backend APIs are involved, verify the frontend path still matches the HTTP handler surface.

## Validation

- Run `npm run build` in `web` for production-safety checks when possible.
- For route or auth work, manually inspect affected router guards and API interceptors.
- Mention if you could not run the frontend build.

## Repo-specific guardrails

- The console is a Vite app under `web/`, not a monorepo package.
- The current app uses Vue Router, Pinia, Axios, Element Plus, and global stylesheet entry `web/src/styles/main.css`.
- `web/README.md` is still mostly the default Vite template, so prefer the source tree over that file when documenting actual behavior.
