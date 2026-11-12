# Configuration

Configuration is stored in `~/.cmdban.yaml` and created on first launch. Use `:config` to edit it in `$EDITOR`.

```yaml
current_board: personal

boards:
  - name: personal
    directory: /Users/you/tasks/personal
    columns:
      - name: today
      - name: tomorrow
      - name: backlog
      - name: done

  - name: work
    directory: /Users/you/tasks/work
    columns:
      - name: in-progress
      - name: review
      - name: blocked
      - name: done
        hidden: true

live_reload_seconds: 30
hide_topbar: false
ignore_files:
  - CLAUDE.md
```

| Option | Description |
|---|---|
| `current_board` | Board shown on launch |
| `boards[].directory` | Where the board's task files live. Use an absolute path: `~` is not expanded, and relative paths resolve from the directory cmdban is started in |
| `boards[].columns` | Lanes, in order; `hidden: true` keeps a column out of view |
| `boards[].type` / `azuredevops` | Azure DevOps boards — see [Azure DevOps](azure-devops.md) |
| `live_reload_seconds` | Re-sync interval while live reload (`R`) is on |
| `hide_topbar` | Start with the topbar hidden (`T` toggles it) |
| `ignore_files` | Markdown files in a board directory that aren't tasks |

Default columns: `today`, `tomorrow`, `backlog`, `done`.

## Other files

| File | Contents |
|---|---|
| `~/.cmdban-pats.yaml` | Personal access tokens, managed with `:pats` |
| `~/.cmdban-comment-reads.yaml` | Which Azure DevOps comments you have seen |

## Environment

| Variable | Default | Description |
|---|---|---|
| `EDITOR` | `nvim` | Editor for tasks, work items and comments |
