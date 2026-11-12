# Task format

Each task is a markdown file in the board's directory. The title is the first heading; the metadata footer is the lines after the last `---`, one field per line.

```markdown
# Fix websocket reconnect bug

The reconnect handler fails after idle timeout.

- reproduce on staging
- inspect retry loop

---
@status today
@priority 1
@order 2
@marked
@tags backend,api
@due 2026-08-21
```

| Field | Meaning |
|---|---|
| `@status` | Board column |
| `@priority` | Priority (non-negative integer) |
| `@order` | Position within the column |
| `@marked` | Flag the card |
| `@tags` | Comma-separated tags |

Any other `@key value` line or bare `@flag` (like `@due` above) is kept as-is when cmdban rewrites the file, so metadata added by hand or by a newer cmdban is never lost. Files are written atomically (temp file, fsync, rename).

The older single-line footer (`@status:today @order:2 @marked @backend`, where bare `@word`s are tags) is still read; a card switches to the line format the next time cmdban saves it.
