# Azure DevOps Integration

cmdban connects directly to Azure DevOps sprint backlogs. Work items appear as Kanban cards in the terminal. Lanes mirror the team's Kanban board columns (e.g. ToDo, In progress, Review, Test), and moving a card moves the work item to that column in ADO via the REST API.

## What syncs

| Operation | Behaviour |
|-----------|-----------|
| Load board | Fetches current sprint work items via WIQL |
| Move task `H` / `L` | PATCH the board column field and its mapped `System.State` in ADO; if ADO rejects the move the board reloads to show where the item really is |
| Create task `a` | POST new work item in the board's first column (New / ToDo), assigned to the PAT owner, in the team's default area and iteration (or the configured `iteration`) |
| Toggle done `d` | Moves the work item to the last column, or back to the first |
| Edit title `e` | PATCH `System.Title` in ADO |
| Refresh `r` | Re-fetches entire sprint |
| `:adocolumns [board]` | Re-reads the Kanban board columns and rebuilds the lanes |
| Toggle lanes `v` | Switches between Kanban column lanes (default) and one lane per work item state; moving a card in state lanes sets only `System.State` |
| Type filter `0`–`9` | Shows one work item type; `0` shows all |
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

### Columns

After the team is chosen, cmdban reads the Kanban board named in the pasted URL (e.g. `Stories`) and creates one lane per board column. The outgoing (done) column starts hidden.

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

      board_level: Stories
      board_column_field: WEF_A75FEBC061DF4941B6154C10660F0B84_Kanban.Column
      board_columns:
        - lane: todo
          column: ToDo
          states: { User Story: New, Bug: New }
        - lane: in-progress
          column: In progress
          states: { User Story: Active, Bug: Active }
        - lane: review
          column: Review
          states: { User Story: Active, Bug: Active }
        - lane: closed
          column: Closed
          states: { User Story: Closed, Bug: Closed }

    columns:
      - name: todo
      - name: in-progress
      - name: review
      - name: closed
        hidden: true
```

You rarely write `board_columns` by hand: run `:adocolumns` on the board and cmdban fills in `board_level`, `board_column_field`, `board_columns` and `columns` from ADO. Pass a board name to switch boards, e.g. `:adocolumns Stories`. Hidden flags on lanes that already exist are kept.

### `iteration`

Use `@CurrentIteration` to always track the team's active sprint, or specify a path directly:

```yaml
iteration: "Platform\\Sprint 42"
```

### `board_columns`

Maps each cmdban lane to a Kanban board column, plus the state each work item type must be in for that column. Several columns can share a state (In progress and Review are both `Active` above), which is why cmdban tracks the column itself rather than the state:

- **Reading** — a card's lane comes from `System.BoardColumn`. Items that aren't on this board (other teams' boards, or types such as Task) fall back to the first column whose state mapping matches, then to the first column.
- **Moving** — cmdban writes both `board_column_field` and the mapped `System.State`. Moving a work item type that the board doesn't carry fails with an error.

### `column_map` (legacy)

Older boards without `board_columns` map lanes to work item states directly. Run `:adocolumns` to upgrade them. Maps cmdban column names (left) to ADO work item states (right). Adjust right-hand values to match your process. Common examples:

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
