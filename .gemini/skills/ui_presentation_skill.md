# UI Presentation Skill (Dashboard)

## Design Aesthetics
- **Color Palette**: Use CSS variables for a modern, sleek look.
  - `--accent-blue`: #3b82f6 (Primary actions, heartbeats)
  - `--accent-green`: #10b981 (Success, online)
  - `--accent-red`: #ef4444 (Errors, offline)
- **Typography**: 
  - Log text and code snippets should use `'JetBrains Mono', 'Fira Code', monospace`.
  - Event messages should be secondary to the metadata (12px font size).

## Log Viewer Patterns
- **Layout**: Use `display: flex` with a fixed-width gutter for line indices.
- **Meta Block**: Group timestamp and level in a `.log-meta` container with `flex-shrink: 0` and `white-space: nowrap` to prevent layout collapse.
- **Alignment**: Use `align-items: baseline` or `center` for multi-spanning log lines.
- **Scrolling**: Use `overflow-y: auto` with custom thin scrollbars.

## Event Feed Patterns
- **Timeline Layout**: Use a dot-and-line timeline style.
  - `.event-node`: Parent container.
  - `.event-node-aside`: Contains the dot and the vertical line.
  - `.event-node-main`: Contains metadata and message.
- **Visual Feedback**: Use color-coded dots (`is-green`, `is-blue`) based on event types.

## Pagination & i18n
- **Chinese Translation Hacks**: For Element Plus components where i18n is stubborn:
  - Use `::before` pseudo-elements on `.el-pagination__jump` to force "跳转至" text.
  - Hide default "Total" and render a custom `.pagination-info` block.
- **State Integration**: Always bind `currentPage`, `pageSize`, and `total` to the monitoring components.
