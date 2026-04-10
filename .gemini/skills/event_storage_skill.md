# Event Storage Skill (BoltDB)

## Overview
The event monitoring system uses BoltDB (`bbolt`) for high-performance indexing and retrieval of millions of events. This avoids the overhead of parsing large JSON files and enables sub-millisecond multi-dimensional filtering.

## Key Buckets
- `events`: Stores the raw JSON event data. Key is `uint64` sequence ID.
- `idx_time`: Time-based index. Key: `Time(UTC)|ID`.
- `idx_type`: Event type index. Key: `Type|Time(UTC)|ID`.
- `idx_service`: Service-based index. Key: `Service|Time(UTC)|ID`.

## Indexing Conventions
1. **UTC Standard**: All timestamps must be converted to UTC before indexing.
2. **Fixed Format**: Use `2006-01-02T15:04:05.000Z` for keys to ensure lexicographical sorting works correctly.
3. **Separator**: Use the pipe `|` character to separate segments in index keys.
4. **Sequence IDs**: Use `BigEndian` encoded `uint64` for raw event keys to maintain stable IDs.

## Query Patterns
- **Range Scans**: Use `Cursor.Seek` with `startTime` and `endTime`.
- **Latest First**: Iterate backwards using `Cursor.Prev()`.
- **Prefix Matching**: Ensure `strings.HasPrefix(k, prefix)` is checked when using type or service buckets.
- **Time Comparison**: When comparing `timePart` strings, ensure `endTime` is padded (e.g., `endTime + "Z\xff"`) to include all milliseconds of the last second.
