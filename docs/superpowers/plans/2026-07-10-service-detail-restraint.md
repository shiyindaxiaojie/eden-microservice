# Service Detail Restraint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce visual noise in the service detail page while preserving its existing filtering, instance actions, pagination, and navigation behavior.

**Architecture:** Keep the existing Vue component and state model. Adjust only the template grouping and its scoped SCSS: move the back action into the title metadata, turn statistics into borderless summaries, and let the instance table sit directly on the page surface.

**Tech Stack:** Vue 3, TypeScript, Element Plus, scoped SCSS, Vite.

## Global Constraints

- Preserve the service group query parameter when returning to `/services`.
- Do not change registry API calls, instance status behavior, i18n copy keys, or RBAC behavior.
- Use visual hierarchy and spacing before introducing any decorative border.

---

### Task 1: Simplify service identity and status summary

**Files:**
- Modify: `web/src/views/service-detail.vue:192-230`
- Test: `web` production build

**Interfaces:**
- Consumes: `goBack()`, `serviceName`, `serviceGroup`, `namespace`, `stats`, `statusFilter`
- Produces: The same status-filter click behavior with less visual chrome.

- [ ] **Step 1: Change the header template so the back action is a text link above the service name**

```vue
<button type="button" class="back-link" @click="goBack">
  <el-icon><ArrowLeft /></el-icon>
  <span>{{ text('返回服务列表', 'Back to services') }}</span>
</button>
<h1>{{ serviceName }}</h1>
<dl class="service-meta">
  <div><dt>{{ text('分组', 'Group') }}</dt><dd>{{ serviceGroup }}</dd></div>
  <div><dt>{{ text('命名空间', 'Namespace') }}</dt><dd>{{ namespace }}</dd></div>
</dl>
```

- [ ] **Step 2: Replace the bordered statistics container with four standalone summary buttons**

```vue
<div class="service-health" aria-label="服务实例统计">
  <button type="button" class="health-stat" :class="{ active: statusFilter === 'all' }" @click="statusFilter = 'all'">
    <strong>{{ stats.total }}</strong><span>{{ text('全部实例', 'All instances') }}</span>
  </button>
</div>
```

- [ ] **Step 3: Scope the header styling to text hierarchy rather than containers**

```scss
.back-link { border: 0; background: transparent; color: var(--text-muted); }
.service-meta { display: flex; gap: 18px; margin: 0; }
.service-health { border: 0; border-radius: 0; background: transparent; }
.health-stat { border: 0; background: transparent; }
```

- [ ] **Step 4: Run the production build**

Run: `cd web; npm run build`

Expected: `vite build` completes without TypeScript or template errors.

### Task 2: Remove the instance-directory card treatment

**Files:**
- Modify: `web/src/views/service-detail.vue:232-340, 479-635`
- Test: `web` production build

**Interfaces:**
- Consumes: The existing `directory-header`, `directory-controls`, `instance-table-wrap`, and `directory-footer` elements.
- Produces: The identical instance list and controls without an enclosing card border or rounded panel.

- [ ] **Step 1: Retain the existing instance-directory markup and remove only its card-surface styling**

```scss
.instance-directory {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  overflow: hidden;
  background: transparent;
}
```

- [ ] **Step 2: Preserve only functional separators for controls, table header/rows, and footer**

```scss
.directory-controls { border-top: 1px solid rgba(148, 181, 207, 0.08); }
.directory-footer { border-top: 1px solid var(--border-color); }
```

- [ ] **Step 3: Run i18n and production checks**

Run: `cd web; npm run check:i18n; npm run build`

Expected: both commands exit with code 0.

- [ ] **Step 4: Inspect the page manually**

Confirm that the title is the primary focal point, group and namespace labels are explicit and non-duplicated, no large card surrounds the instance list, and clicking each count still filters the table.
