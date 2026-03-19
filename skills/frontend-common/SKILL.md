---
name: frontend-common
description: Detailed Vue 3 frontend development guidelines, project structure, layout conventions, dialog rules, API placement rules, and styling constraints for eden-go-registry. Use when working in web/ to implement or review views, routes, API modules, dashboard pages, dialogs, tables, charts, or shared styling behavior.
---

# Frontend Common Skill

This skill defines the baseline frontend rules for `eden-go-registry`.

## 1. Tech Stack

- Framework: Vue 3 + TypeScript + Vite.
- UI library: Element Plus.
- State management: Pinia.
- Routing: Vue Router.
- Charts: ECharts.
- Styling: shared global CSS in `web/src/styles/main.css`.

## 2. Project Structure (`web/src/`)

- `api/`: Axios-based API modules.
- `views/`: page components.
- `router/`: route definitions.
- `utils/`: i18n and other helpers.
- `styles/`: global styles, theme tokens, and shared visual patterns.
- `App.vue`: app shell, sidebar, header, theme switching, and navigation.

## 3. Styling Rules

- Preserve the frosted-glass, dark-first visual language already used by the app.
- Avoid flat white cards or borders that break the existing theme.
- Put shared styles in `web/src/styles/main.css`; do not duplicate reusable layout styles inside each component.
- Reuse existing CSS variables such as `--bg-card`, `--border-color`, `--accent-blue`, and related tokens.
- Prefer semantic or BEM-style class names.
- Preserve both dark and light theme behavior because the app toggles `data-theme` on the root element.

### 3.1 Sub-page Layout Rules

When building sub-pages such as detail pages, management screens, or secondary lists, follow the existing shell structure and keep the page layout visually consistent:

```html
<div class="app-container infrastructure-page">
  <div class="app-panel">
    <div class="page-header">
      <div class="header-main">
        <div class="header-left">
          <h1 class="page-title">
            <el-icon><YourIcon /></el-icon> Title
          </h1>
        </div>
        <div class="header-right category-switcher">
          <div class="category-btn" @click="goBack">
            <el-icon><ArrowLeft /></el-icon><span>返回上级</span>
          </div>
        </div>
      </div>
      <p class="page-desc">Description of the page</p>
    </div>

    <div class="main-nav-container">
      <div class="category-tabs"></div>
      <div class="toolbar-actions">
        <el-form :inline="true" class="header-filter-form">
          <!-- form items -->
        </el-form>
        <el-button type="primary" class="add-btn">
          <el-icon><Plus /></el-icon><span>新增</span>
        </el-button>
      </div>
    </div>

    <div class="content-body">
      <div class="table-container">
        <el-table class="middleware-table" height="100%">...</el-table>
      </div>
      <div class="middleware-pagination-container">
        <el-pagination class="middleware-pagination" />
      </div>
    </div>
  </div>
</div>
```

- Prefer shared layout classes from the global stylesheet when possible.
- Avoid writing one-off component CSS for standard page layout if the global styles already cover it.

### 3.2 Dialog Rules

- Always add `append-to-body` to `el-dialog` so overlays center correctly and are not constrained by parent layout containers.

```html
<el-dialog v-model="visible" title="Example" append-to-body>
  ...
</el-dialog>
```

## 4. API and Error Handling

- Define API calls in `web/src/api/<module>/` or the existing submodule structure.
- Reuse the shared Axios instance in `web/src/api/index.ts` for JWT injection and `401` redirect behavior.
- Keep frontend response expectations aligned with backend JSON contracts.
- When adding new pages, update route registration and any matching navigation entries together.
- When adding new user-facing labels, update the i18n source instead of hardcoding copy in multiple places.

## 5. Development Workflow

- Run `npm run dev` for local development.
- Run `npm run build` after non-trivial frontend changes when local dependencies are already available.
- Verify route guards, theme switching, dialogs, and navigation after UI changes.