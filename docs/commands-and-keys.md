# Commands and keys

Press `?` inside cmdban for the same list, grouped by view.

## Board

| Key | Action |
|---|---|
| `h` / `l` | Left / right lane |
| `j` / `k` | Up / down |
| `gg` / `G` | Top / bottom of lane |
| `ctrl+u` / `ctrl+f` | Half page up / down |
| `enter` | View task |
| `tab` | Focus mode |
| `a` | New task |
| `e` / `i` | Edit title |
| `ctrl+g` | Open task in `$EDITOR` |
| `d` | Toggle done |
| `t` | Add tag |
| `m` | Mark task |
| `x` / `ctrl+d` | Delete task |
| `H` / `L` | Move task left / right |
| `ctrl+k` / `ctrl+j` | Reorder task up / down |
| `/` | Search titles and tags (`esc` clears) |
| `.` | Minimize the lane |
| `r` | Refresh |
| `R` | Toggle live reload: re-syncs every 30s (`live_reload_seconds`); the topbar's Synced line turns red while on |
| `T` | Toggle the topbar (remembered as `hide_topbar`) |
| `ctrl+b` | Board switcher |
| `ctrl+n` | Next recent board |
| `ctrl+m` | Move task to another board |
| `:` | Command mode |
| `?` | Help |
| `q` | Quit |

## Azure DevOps boards

| Key | Action |
|---|---|
| `b` | New Bug, created like `a` (first column, assigned to you) |
| `0` | Default view: User Stories and Bugs |
| `1`–`6` | A single work item type, as numbered in the topbar's FILTERS section |
| `7` / `U` | Toggle unassigned cards from your team's area on an assigned-to-me board (saved as `include_unassigned`) |
| `8` | Cards with unread comments |
| `9` | All types |
| `v` | Toggle lanes between Kanban board columns and work item states |
| `c` | Comments view; `a` there writes a comment in `$EDITOR` |
| `M` | Open the work item in `$EDITOR` as read-only markdown, comments included |
| `ctrl+o` | Open the work item in the browser |
| `O` | Open the board in the browser |

Cards show a type glyph: ◆ story, ● bug, ▪ task, ★ feature, ♛ epic.

Cards with comments added since you last opened them show 💬 before the title. Opening the comments view or pane marks them read; the read state lives in `~/.cmdban-comment-reads.yaml`, and a work item's existing comments count as read the first time it syncs.

## Task view

| Key | Action |
|---|---|
| `j` / `k` | Scroll |
| `tab` | Toggle the side pane: linked PRs (status, approvals, comments, pipeline runs) above the comments |
| `J` / `K` | Scroll the comments pane |
| `l` | Link one of your own PRs (any project in the org) |
| `c` | Comments view |
| `H` / `L`, `e`, `d`, `x` | Move, edit, toggle done, delete — as on the board |
| `q` | Back |

## Focus mode

| Key | Action |
|---|---|
| `j` / `k`, `h` / `l` | Select card / lane |
| `enter` | Open card |
| `tab` / `q` | Exit focus |

Most board keys (`a`, `b`, `e`, `t`, `m`, `c`, `M`, `H` / `L`) work here too.

## Board switcher

| Key | Action |
|---|---|
| `j` / `k` | Navigate |
| `enter` | Switch to board |
| `a` | New local board |
| `A` | New Azure DevOps board (guided setup) |
| `q` / `esc` | Close |

## Commands

| Command | Action |
|---|---|
| `:boards` / `:b` | Board switcher |
| `:pats` / `:p` | PAT manager |
| `:config` / `:c` | Open config in `$EDITOR` |
| `:settings dir <path>` | Set the board's task directory |
| `:adocolumns [board]` | Sync lanes from an Azure DevOps Kanban board |
| `:move` / `:m` | Move task to another board |
| `:clear` | Archive all tasks in the lane |
| `:logs` / `:l` | Log viewer |
| `:help` / `:h` | Help |
| `:quit` / `:q` | Quit |
