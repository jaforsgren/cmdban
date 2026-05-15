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

Tasks are markdown files with a metadata footer.

```markdown
# Fix websocket reconnect bug

The reconnect handler fails after idle timeout.

- reproduce on staging
- inspect retry loop

---
@status:today @priority:1 @backend
```

`@status:` maps the task to a board column. Default columns: `today`, `tomorrow`, `backlog`, `done`.

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

Open the board switcher with `:b` or `ctrl+b`.

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

### Commands

| Command | Action |
|---------|--------|
| `:` | Command mode |
| `:b` | Board switcher |
| `:pats` / `:p` | PAT manager |
| `:config` / `:c` | Open config in editor |
| `:set dir <path>` | Set task directory |
| `:q` | Quit |
| `?` | Help |

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
