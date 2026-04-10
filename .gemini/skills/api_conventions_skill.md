# Pagination & API Skill

## Backend Query Params
Standard API endpoints for large datasets (Events, Logs) must support:
- `count` (int): Number of items per page.
- `offset` (int): Number of items to skip.
- `query` (string): Text search.
- `start_time` / `end_time`: ISO UTC range strings.
- `type` / `service`: Category filters.

## Response Format
Always return an object containing the total count to support frontend pagination:
```json
{
  "total": 5420,
  "data": [...]
}
```

## Logic
- **Reverse Iteration**: For monitoring data, items should typically be returned latest-first. Offset should skip the N most recent items.
- **Count First**: Always calculate the filtered total before fetching the slice to populate the pagination component correctly.
- **Batching**: Use a default limit (e.g., 100) to prevent OOM when users request data without a count parameter.
