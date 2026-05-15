# Azure DevOps Integration

cmdban connects directly to Azure DevOps sprint backlogs. Work items appear as Kanban cards in the terminal. Moving a card between columns updates the work item state in ADO via the REST API.

## What syncs

| Operation | Behaviour |
|-----------|-----------|
| Load board | Fetches current sprint work items via WIQL |
| Move task `H` / `L` | PATCH `System.State` in ADO |
| Create task `a` | POST new work item in ADO |
| Edit title `e` | PATCH `System.Title` in ADO |
| Refresh `r` | Re-fetches entire sprint |
| Delete, tags, reorder | Blocked — manage in ADO web UI |

---

## Step 1 — Create a PAT

Go to:

```
https://dev.azure.com/<your-org>/_usersSettings/tokens
```

Click **New Token**. Required scope:

- **Work Items** → Read & Write

Copy the token value.

---

## Step 2 — Store the PAT in cmdban

Run `:pats` inside cmdban.

Press `a` to add an entry:

- **Name** — a short alias you choose, e.g. `myorg`. This is what your board config references, not the token itself.
- **Token** — paste the PAT (input is masked)

PATs are stored in `~/.cmdban-pats.yaml` with `0600` permissions. They are never written to the main config file.

PAT manager keys:

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate |
| `a` | Add PAT |
| `x` | Delete selected PAT |
| `space` | Reveal / hide token |
| `q` / `Esc` | Close |

You can store multiple PATs for different organisations.

---

## Step 3 — Add an ADO board (interactive wizard)

Open the board switcher with `:b` and press **`A`** (capital A).

The setup wizard walks through five steps:

### PAT selection

All stored PATs are listed. Select the one for your organisation.

### Organisation

cmdban attempts to auto-discover organisations accessible to your PAT using the ADO profile API. If multiple orgs are found, you pick from the list. If discovery fails (common with org-scoped PATs), a text prompt appears instead.

### Project

Projects are fetched from the selected organisation. Pick from the list.

### Team

Teams are fetched from the selected project. Pick from the team you want to track.

### Board name

A default name is suggested based on the project name (e.g. `platform-sprint`). Edit it or accept as-is.

On confirm, the board is written to `~/.cmdban.yaml` and activated immediately.

---

## Manual config

You can also add ADO boards directly in `~/.cmdban.yaml`. Open it with `:config`.

```yaml
boards:
  - name: backend-sprint
    type: azuredevops

    azuredevops:
      org: myorg
      project: Platform
      team: Backend
      iteration: "@CurrentIteration"
      pat: myorg
      default_work_item_type: "User Story"

      column_map:
        today: "Active"
        tomorrow: "Committed"
        backlog: "New"
        done: "Closed"

    columns:
      - name: today
      - name: tomorrow
      - name: backlog
      - name: done
```

### `iteration`

Use `@CurrentIteration` to always track the team's active sprint, or specify a path directly:

```yaml
iteration: "Platform\\Sprint 42"
```

### `column_map`

Maps cmdban column names (left) to ADO work item states (right). Adjust right-hand values to match your process. Common examples:

| ADO process | States |
|-------------|--------|
| Agile | `New`, `Active`, `Resolved`, `Closed` |
| Scrum | `New`, `Approved`, `Committed`, `Done` |
| CMMI | `Proposed`, `Active`, `Resolved`, `Closed` |

### `pat`

The alias of a stored PAT entry, not the token itself.

### `default_work_item_type`

Work item type used when creating tasks from cmdban. Defaults to `User Story`.

---

## Multiple ADO boards

Each board is independent. You can mix local and ADO boards freely.

```yaml
current_board: backend-sprint

boards:
  - name: personal
    directory: ~/tasks/personal
    columns:
      - name: today
      - name: backlog
      - name: done

  - name: backend-sprint
    type: azuredevops
    azuredevops:
      org: myorg
      project: Platform
      team: Backend
      iteration: "@CurrentIteration"
      pat: myorg
      column_map:
        today: "Active"
        backlog: "New"
        done: "Closed"
    columns:
      - name: today
      - name: backlog
      - name: done

  - name: client-work
    type: azuredevops
    azuredevops:
      org: clientorg
      project: ClientApp
      team: ClientTeam
      iteration: "@CurrentIteration"
      pat: clientorg
      column_map:
        today: "In Progress"
        backlog: "To Do"
        done: "Done"
    columns:
      - name: today
      - name: backlog
      - name: done
```

---

## Switching boards

Open the board switcher with `:b`. ADO boards show an `[ADO]` badge and display `org / project / team` instead of a directory path.

```
▶ backend-sprint  [ADO] ✓
    myorg / Platform / Backend
  personal
    ~/tasks/personal
```

Press `Enter` to switch. cmdban fetches the sprint immediately and shows sync time in the header (`synced 12s ago`). Press `r` to re-sync at any time.

---

## Troubleshooting

| Error | Fix |
|-------|-----|
| `PAT "myorg" not found — run :pats to add it` | Run `:pats` and add an entry with that alias |
| `ADO 401` | PAT expired or has wrong scopes — delete and re-add via `:pats` |
| `ADO 404` | Check `org`, `project`, or `team` values |
| `no current iteration found for team "..."` | Verify the team has an active sprint in ADO, or set a fixed `iteration` path |
| `fetching organisations: ADO 203` | PAT is org-scoped — org discovery failed, type the org name manually when prompted |
| `resolving iteration: ...` | The `team` field must match the ADO team name exactly |
