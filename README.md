A K9s-inspired terminal Kanban board for developers.

Tasks are plain markdown files. Navigate with vim-ish keybindings. Use it locally, commit tasks to git, or connect directly to Azure DevOps sprint backlogs.

cmdban is built around a few ideas:

- text files over databases
- keyboard-first workflows
- local-first tooling
- fast startup and navigation

---

## Features

- Vim-style navigation (`hjkl`, `gg`, `G`, `ctrl+u`, `ctrl+d`)
- Tasks stored as plain markdown files
- Configurable board columns
- Markdown preview rendering
- Open tasks in `$EDITOR` (`nvim` by default)
- Multiple boards with independent task directories
- Azure DevOps sprint backlog sync

---

## Installation

```bash
git clone <repo>
cd cmdban
make build
make install   # optional, installs to PATH
```

---

## Quick start

```bash
cmdban
```

- `a` — create task
- `H` / `L` — move task between columns
- `Enter` — open task detail
- `?` — show all keybindings

---

## Task format

Tasks are markdown files with a metadata footer: the lines after the last `---`, one field per line.

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
|-------|---------|
| `@status` | Board column |
| `@priority` | Priority (non-negative integer) |
| `@order` | Position within the column |
| `@marked` | Flag the card |
| `@tags` | Comma-separated tags |

Any other `@key value` line or bare `@flag` (like `@due` above) is kept as-is when cmdban rewrites the file, so metadata added by hand or by a newer cmdban is never lost. Files are written atomically (temp file, fsync, rename).

The older single-line footer (`@status:today @order:2 @marked @backend`, where bare `@word`s are tags) is still read; a card switches to the line format the next time cmdban saves it.

Default columns: `today`, `tomorrow`, `backlog`, `done`.

---

## Configuration

Stored in `~/.cmdban.yaml`. Open it from inside cmdban with `:config`.

```yaml
current_board: personal

boards:
  - name: personal
    directory: ~/tasks/personal
    columns:
      - name: today
      - name: tomorrow
      - name: backlog
      - name: done

  - name: work
    directory: ~/tasks/work
    columns:
      - name: in-progress
      - name: review
      - name: blocked
      - name: done
```

---

## Boards

Open the board switcher with `:b` or `ctrl+b`. The topbar's RECENT section lists the last four boards you used; `ctrl+n` cycles through them.

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate |
| `Enter` | Switch to board |
| `a` | New local board |
| `A` | New Azure DevOps board (guided setup) |
| `q` / `Esc` | Close |

---

## Azure DevOps

cmdban can connect to ADO sprint backlogs. Work items appear as cards and changes (moves, new tasks, title edits) sync back via the REST API.

See **[docs/azure-devops.md](docs/azure-devops.md)** for full setup instructions, including:

- creating and storing PATs (`:pats`)
- the interactive board setup wizard (`A` in the board switcher)
- manual config reference
- column state mapping
- troubleshooting

---

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `h` / `l` | Left / right lane |
| `j` / `k` | Up / down |
| `gg` | Top of lane |
| `G` | Bottom of lane |
| `ctrl+u` / `ctrl+f` | Half page up / down |

### Task actions

| Key | Action |
|-----|--------|
| `Enter` | View task |
| `a` | New task |
| `e` / `i` | Edit title |
| `d` | Toggle done |
| `t` | Add tag |
| `x` / `ctrl+d` | Delete task |
| `H` / `L` | Move task left / right |
| `ctrl+k` / `ctrl+j` | Reorder task up / down |
| `m` | Mark task |
| `/` | Search |
| `r` | Refresh |
| `ctrl+n` | Next recent board |
| `T` | Toggle the topbar (remembered as `hide_topbar` in the config) |
| `R` | Toggle live reload (off by default): re-syncs every 30s, or `live_reload_seconds` in the config; the topbar's Synced line turns red while on |

### Azure DevOps boards

| Key | Action |
|-----|--------|
| `b` | New Bug, created like `a` (first column, assigned to you) |
| `0` / `1`–`8` / `9` | Work item type filter: `0` (default) shows User Stories and Bugs, `1`–`8` a single type as numbered in the topbar's TYPES section, `9` all types. Cards show a small type glyph: ◆ story, ● bug, ▪ task, ★ feature, ♛ epic |
| `U` | Toggle unassigned cards from your team's area on an assigned-to-me board (saved as `include_unassigned`) |
| `v` | Toggle lanes between Kanban board columns and work item states |
| `ctrl+o` | Open the selected work item in the browser |
| `O` | Open the ADO board in the browser |
| `M` | Open the selected work item in `$EDITOR` as markdown: title, type, lane, tags, link, description, repro steps and comments (a temporary read-only copy) |
| `c` / `ctrl+c` | Comments view (the topbar shows the selected card's comment count) |
| `a` (in comments view) | Write a comment in `$EDITOR`; save and quit posts it as markdown, an empty file discards it |
| `tab` (in task view) | Toggle a side pane beside the work item details: linked PRs (link, status, approvals, comment count, description excerpt, pipeline runs of the merge commit) above the comments |
| `l` (in task view) | Link one of your own PRs (any project in the org) to the work item |
| `J` / `K` (in task view) | Scroll the comments pane |

### Commands

| Command | Action |
|---------|--------|
| `:` | Command mode |
| `:b` | Board switcher |
| `:pats` / `:p` | PAT manager |
| `:config` / `:c` | Open config in editor |
| `:set dir <path>` | Set task directory |
| `:adocolumns [board]` | Sync lanes from an ADO Kanban board |
| `:q` | Quit |
| `?` | All shortcuts and commands |

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EDITOR` | `nvim` | Editor for opening tasks |

---

## Development

```bash
make build
make run
make test
make lint
make tidy
```
