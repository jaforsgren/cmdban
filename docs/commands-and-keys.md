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
| `t` | Add tag — lists the board's tags matching what you type (all of them before you type); `↑`/`↓` or `ctrl+n`/`ctrl+p` pick, `tab` completes, `enter` adds |
| `m` | Mark task |
| `x` / `ctrl+d` | Delete task |
| `H` / `L` | Move task left / right |
| `ctrl+k` / `ctrl+j` | Reorder task up / down |
| `/` | Search titles and tags (`esc` clears) |
| `.` | Minimize the lane |
| `r` | Refresh |
| `R` | Toggle live reload: re-syncs every 30s (`live_reload_seconds`); the topbar's Synced line turns red while on |
| `T` | Toggle the topbar (remembered as `hide_topbar`) |
| `ctrl+b` | Board switcher (`a` new local, `A` new ADO, `e` edit selected, `r` rename selected) |
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

## Wiki mode

`:wiki` (or `:w`) switches to wiki mode, which lists your Azure DevOps wiki pages and nothing else — boards stay out of it. Each wiki is one line; open it to see the pages beneath it as a tree, with lines showing which page sits under which and a count of sub pages beside every parent.

| Key | Action |
|---|---|
| `j` / `k`, `gg` / `G` | Select page |
| `l` / `→` | Expand the page (▸ → ▾); on an expanded page, step into its first sub page |
| `h` / `←` | Collapse the page; on a collapsed page, step out to its parent |
| `enter` | Read the page full screen (`j` / `k` scroll, `q` back) |
| `tab` | Toggle a preview of the selected page beside the tree (`ctrl+u` / `ctrl+f` scroll it) |
| `ctrl+g` | Edit the page in `$EDITOR`; on save you're asked before the change is published to Azure DevOps |
| `M` | Open the page read-only in `$EDITOR` |
| `ctrl+o` | Open the page in the browser |
| `r` | Refresh every wiki |
| `a` | Add a wiki: paste a page URL from the browser |
| `x` | Remove the selected wiki from cmdban (its top line only; nothing changes in Azure DevOps) |
| `q` / `esc` | Back to the board |

Edits are guarded by the page's version: if someone changed the page after you opened it, publishing is refused and your edit is left in the temp file named in the message.

## Commands

| Command | Action |
|---|---|
| `:boards` / `:b` | Board switcher |
| `:pats` / `:p` | PAT manager |
| `:config` / `:c` | Open config in `$EDITOR` |
| `:settings dir <path>` | Set the board's task directory |
| `:adocolumns [board]` | Sync lanes from an Azure DevOps Kanban board |
| `:wiki` / `:w [url]` | Wiki mode; with a URL, add that wiki page |
| `:move` / `:m` | Move task to another board |
| `:clear` | Archive all tasks in the lane |
| `:logs` / `:l` | Log viewer |
| `:help` / `:h` | Help |
| `:quit` / `:q` | Quit |
