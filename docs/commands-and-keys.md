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
| `x` / `ctrl+d` | Delete task (asks `y`/`n`) |
| `x` then `1`–`9` | Toggle that checkbox on the card; the task view shows each checkbox's number |
| `u` / `ctrl+r` | Undo / redo moves, edits, tags, marks, reordering, archiving, creating, and moves to another board. History lasts for the session, per board, and undo refuses to overwrite a card changed since |
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
| `yp` | Copy the work item link to the clipboard |
| `yr` | Copy the linked PRs to the clipboard: the card title, then one `repository: link` line per PR |
| `O` | Open the board in the browser |
| `ctrl+s` | Save the card's checkbox changes to Azure DevOps |
| `x` then `u` | Discard the card's unsaved checkbox changes |

Cards show a type glyph: ◆ story, ● bug, ▪ task, ★ feature, ♛ epic.

Checkbox toggles on a work item stay local until saved, shown by a `*` after the card's count such as `2/4*`. They are kept in `~/.cmdban-checkbox-drafts.yaml`, so they survive refreshes and restarts.

Cards with comments added since you last opened them show 💬 before the title. Opening the comments view or pane marks them read; the read state lives in `~/.cmdban-comment-reads.yaml`, and a work item's existing comments count as read the first time it syncs.

## Visual select

Press `V` on the board to select a range. Cards marked with `m` are included too, across lanes. Each action below is one `u` undo step.

| Key | Action |
|---|---|
| `j` / `k` | Extend the range |
| `m` | Move to a lane, typed by name |
| `t` | Add a tag |
| `p` | Set priority (local boards) |
| `a` / `d` | Archive, after `y`/`n` (local boards) |
| `esc` / `V` | Exit |

On Azure DevOps boards, moves and tags ask for `y`/`n` before writing.

## Task view

| Key | Action |
|---|---|
| `j` / `k` | Scroll |
| `tab` | Toggle the side pane: linked PRs (status, approvals, comments, pipeline runs) above the comments |
| `J` / `K` | Scroll the comments pane |
| `l` | Link one of your own PRs (any project in the org) |
| `c` | Comments view |
| `H` / `L`, `e`, `d`, `x`, `yp`, `yr` | Move, edit, toggle done, delete, copy link, copy PRs — as on the board |
| `q` | Back |

## Focus mode

| Key | Action |
|---|---|
| `j` / `k`, `h` / `l` | Select card / lane |
| `enter` | Open card |
| `tab` / `q` | Exit focus |

Most board keys (`a`, `b`, `e`, `t`, `m`, `c`, `M`, `yp`, `yr`, `H` / `L`) work here too.

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
| `z` | Expand or fold the selected page, keeping the selection |
| `Z` | Expand the selected page and everything beneath it; when all of it is already open, fold it all |
| `enter` | Read the page full screen (`j` / `k` scroll, `q` back) |
| `tab` | Toggle a preview of the selected page beside the tree (`ctrl+u` / `ctrl+f` scroll it) |
| `ctrl+g` | Edit the page in `$EDITOR`; on save you're asked before the change is published to Azure DevOps |
| `M` | Open the page read-only in `$EDITOR` |
| `ctrl+o` | Open the page in the browser |
| `space` `space` | Find pages: fuzzy search every page's breadcrumb, like telescope's find files |
| `space` `s` `g` | Grep pages: search every line of every page (reads all pages the first time; ignores case unless you type an upper case letter) |
| `r` | Refresh every wiki |
| `R` | Toggle watch: refresh the wikis and the pages you've opened every 30s (`live_reload_seconds`); the Synced line turns red while on |
| `a` | Add a wiki: paste a page URL from the browser |
| `x` | Remove the selected wiki from cmdban (its top line only; nothing changes in Azure DevOps) |
| `q` / `esc` | Back to the board |

In find and grep, type to filter; `↑`/`↓`, `ctrl+n`/`ctrl+p` or `ctrl+j`/`ctrl+k` pick a result, `ctrl+u`/`ctrl+d` scroll the preview, `enter` opens the page (a grep hit opens at the matching line, with the page revealed in the tree behind it) and `esc` closes. Page titles containing what you typed rank first.

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
