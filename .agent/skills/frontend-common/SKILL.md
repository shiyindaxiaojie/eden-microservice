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

### 3.1 Sub-page UI Layout Specification

When building sub-pages (like detail pages or secondary lists), strictly follow this layout structure:

```html
<div class="app-container infrastructure-page">
  <div class="app-panel">
    <!-- 1. Page Header -->
    <div class="page-header">
      <div class="header-main">
        <div class="header-left">
          <h1 class="page-title">
            <el-icon><YourIcon /></el-icon> Title
          </h1>
        </div>
        <!-- Right side actions (e.g. Back button) -->
        <div class="header-right category-switcher">
          <div class="category-btn" @click="goBack">
            <el-icon><ArrowLeft /></el-icon><span>返回上级</span>
          </div>
        </div>
      </div>
      <p class="page-desc">Description of the page</p>
    </div>

    <!-- 2. Navigation & Toolbar -->
    <div class="main-nav-container">
      <div class="category-tabs"></div>
      <!-- For left-aligned tabs if any -->
      <div class="toolbar-actions">
        <!-- Inline Search Form -->
        <el-form :inline="true" class="header-filter-form">
          <!-- Form items -->
        </el-form>
        <!-- Primary Action Button -->
        <el-button type="primary" class="add-btn"
          ><el-icon><Plus /></el-icon><span>新增</span></el-button
        >
      </div>
    </div>

    <!-- 3. Content & Table -->
    <div class="content-body">
      <div class="table-container">
        <!-- Table with class middleware-table and height="100%" -->
        <el-table class="middleware-table" height="100%">...</el-table>
      </div>
      <!-- Pagination -->
      <div class="middleware-pagination-container">
        <el-pagination class="middleware-pagination" />
      </div>
    </div>
  </div>
</div>
```

- Only use standard classes defined in `main.scss` and `infrastructure.scss`. Avoid writing custom CSS in the vue component for layout if possible.

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
