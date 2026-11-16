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

## Wikis

Wikis shown in wiki mode (`:wiki`) live in their own list, apart from the boards. Adding one with `a` in wiki mode (or `:wiki <url>`) fills it in from the page URL and picks the PAT an ADO board in the same organisation already uses.

```yaml
wikis:
  - name: Kravdokumentation
    org: lfantdevelophub
    project: common
    wiki: ProjectManagement
    page_id: 93
    pat: work
```

| Option | Description |
|---|---|
| `name` | Shown on the wiki's line; the page's title when added |
| `org` / `project` / `wiki` | Where the wiki lives |
| `page_id` | The page shown with everything beneath it, as in the page URL (`.../_wiki/wikis/<wiki>/<page_id>/<slug>`); leave it out for the whole wiki |
| `pat` | Name of the PAT (see `:pats`) used to read and edit the wiki |

## Other files

| File | Contents |
|---|---|
| `~/.cmdban-pats.yaml` | Personal access tokens, managed with `:pats` |
| `~/.cmdban-comment-reads.yaml` | Which Azure DevOps comments you have seen |

## Environment

| Variable | Default | Description |
|---|---|---|
| `EDITOR` | `nvim` | Editor for tasks, work items and comments |
