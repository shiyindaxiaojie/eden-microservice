---
name: frontend-common
description: Core Vue 3 frontend development guidelines, project structure, and styling rules for eden-go-registry.
---

# Frontend Common Skill

This skill covers the foundational rules for frontend development in `eden-go-registry` (web directory).

## 1. Tech Stack

- **Framework**: Vue 3 + TypeScript + Vite.
- **UI Library**: Element Plus.
- **State Management**: Pinia.
- **Styling**: SCSS.

## 2. Project Structure (`web/src/`)

- `api/`: API modules (Axios).
- `views/`: Page components.
- `components/`: Reusable components.
- `store/`: Pinia stores.
- `styles/`: Global styles (`main.scss`).

## 3. Styling Rules

- **Frosted Glass Theme**: The app uses a dark/frosted glass theme. **Avoid white backgrounds and borders.**
- **Common Styles**:
  - **DO NOT** define reusable common styles in components.
  - Use `web/src/styles/main.scss` for shared classes like `.app-container`, `.search-section`, `.table-section`.
- **BEM Naming**: Use BEM or semantic naming for classes.
- **Unified Typography**: Baseline font size is **14px** for maximum readability. Major titles use **18-20px**, while sub-headings and labels use **12-13px**. Metadata uses **11px**.
- **State Persistence**: Manual status changes (e.g., offlining an instance) MUST persist via a `manual_offline` flag to prevent automatic heartbeat-based recovery.

### 3.1 Premium Component Design (Glassmorphism)

All cards and panels should follow the frosted glass aesthetic:
- **Background**: `var(--bg-glass)` or `rgba(255, 255, 255, 0.03)`.
- **Border**: `1px solid var(--border-color)` with rounded corners (12px for major cards, 4px/8px for smaller items).
- **Hover Effects**: Suble lift (`transform: translateY(-4px)`), enhanced shadow, and glow effects (`box-shadow` matching the component's accent color).
- **Smooth Transitions**: Use `transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1)`.

### 3.2 Sub-page UI Layout Specification

When building sub-pages or main management views (Namespace, RBAC, etc.), use the following standardized layout for consistency:

```html
<div class="app-container">
  <div class="app-panel">
    <!-- 1. Page Header (Bold Titles + Stats) -->
    <div class="page-header">
      <div class="header-main">
        <div class="header-left">
          <h1 class="page-title">
            <el-icon><YourIcon /></el-icon> <span>标题</span>
          </h1>
        </div>
        <div class="header-right">
          <div class="stats-group">
            <div class="stat-chip">
              <span class="stat-value">99</span>
              <span class="stat-label">标签</span>
            </div>
          </div>
        </div>
      </div>
      <p class="page-desc">页面描述文本 (14px/muted)</p>
    </div>

    <!-- 2. Toolbar & Filtering -->
    <div class="main-nav-container">
      <div class="toolbar-actions">
        <el-input class="header-search" v-model="search" />
        <el-button class="pill-btn" :icon="Refresh" @click="fetch">刷新</el-button>
        <div class="toolbar-sep"></div>
        <el-button type="primary" class="add-btn" :icon="Plus" @click="add">新增</el-button>
      </div>
    </div>

    <!-- 3. Content Body (Grid or Table) -->
    <div class="content-body">
      <!-- For Tables: wrap in .table-wrap and use height="100%" -->
      <div class="table-wrap">
        <el-table :data="data" height="100%">...</el-table>
      </div>
      
      <!-- For Grids: use .namespace-grid or similar with flex/grid containers -->
      <div class="card-grid">...</div>
    </div>
  </div>
</div>
```

- **Layout Components**:
  - `.app-panel`: Flex container (column) for the entire page.
  - `.page-title`: **20px font-weight: 700**.
  - `.content-body`: Set to `flex: 1` and `min-height: 0` to enable internal scrolling.
  - `.table-wrap`: Rounded corners (12px), border, and overflow: hidden.

### 3.2 Dialog / Popup Specification

- **Append to Body**: To ensure that dialogs (`<el-dialog>`) properly center relative to the entire screen overlay and aren't misaligned or confined by their parent container's layout (e.g. within an `.app-panel`), **always provide the `append-to-body` attribute**.
  ```html
  <el-dialog v-model="visible" title="Example" append-to-body> ... </el-dialog>
  ```

## 4. API & Error Handling

- **Location**: Define API calls in `web/src/api/<module>/`.
- **Error Codes**: Sync with backend codes in `web/src/utils/errorCodes.ts`.
- **Interceptors**: Axios interceptors handle global error display (Element Plus Message).

## 5. Development Workflow

- **Run**: `npm run dev`
- **Build**: `npm run build`
