# Monitoring Time Filter Skill

## Integration Pattern
The dashboard uses a unified `el-date-picker` with `type="datetimerange"` for all temporal queries.

## Shortcut Definitions
Standardize the following relative time shortcuts (UTC based):
- **Last 30m**: `[now - 30m, now]`
- **Last 1h**: `[now - 1h, now]`
- **Last 24h**: `[now - 24h, now]`
- **Last 7 days**: `[now - 7d, now]`

## Implementation Rules
1. **Model**: Bind to an array `[Date, Date] | null`.
2. **Value Transformation**: Always convert to `toISOString()` before making the API call.
3. **Empty State**: If the picker is cleared, `startTime` and `endTime` should be sent as empty strings to indicate "All History" or "Default Window".
4. **Trigger**: Perform `fetchEvents(0)` (reset to page 1) on any `change` event.
