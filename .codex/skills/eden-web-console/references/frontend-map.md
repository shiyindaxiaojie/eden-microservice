# Frontend Map

## Entry points

- `web/src/main.ts`: app bootstrap, Pinia, router, Element Plus
- `web/src/router/index.ts`: route table and auth guard
- `web/src/api/index.ts`: axios instance, JWT injection, `401` handling

## Main views

- `web/src/views/dashboard.vue`
- `web/src/views/services.vue`
- `web/src/views/service-detail.vue`
- `web/src/views/cluster.vue`
- `web/src/views/settings.vue`
- `web/src/views/rbac.vue`
- `web/src/views/login.vue`
- `web/src/views/namespace.vue`
- `web/src/views/profile.vue`
- `web/src/views/docs.vue`

## Build/runtime basics

- Dev command: `npm run dev`
- Build command: `npm run build`
- Default console dev URL from repo docs: `http://127.0.0.1:2019`
- Frontend expects backend HTTP API under `/v1/*`

## When to read more

- Read `README.md` or `docs/user_guide_zh.md` if the task depends on console login flow or backend default addresses.
