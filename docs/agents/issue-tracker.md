# Issue tracker: Linear

Issues and PRDs for this repo live in Linear: team **`GABE`**, project **Yogurt**. The `GABE` team also holds unrelated work, so always scope reads and writes to project `Yogurt`. Use the Linear MCP server's tools for all operations (`linear-server`; tools appear as `mcp__linear-server__*` in Claude Code, `mcp__linear_server__*` in Codex). If those tools aren't available, stop and ask; don't fall back to `gh`.

## Conventions

- **Create an issue**: `save_issue` without `id`, with `team: "GABE"`, `project: "Yogurt"`, `title`, markdown `description`, and any `labels`.
- **Read an issue**: `get_issue` with `id: "GABE-42"` (`includeRelations: true` for blockers), then `list_comments` with `issueId`.
- **List issues**: `list_issues` with `team: "GABE"`, `project: "Yogurt"`, plus `label` / `state` / `assignee` / `parentId` as needed. Paginate with `cursor`.
- **Comment on an issue**: `save_comment` with `issueId` and markdown `body`.
- **Apply / remove labels**: `save_issue` with `id` and `addLabels` / `removeLabels` (`labels` replaces the whole set). Check `list_issue_labels` first; create missing ones with `save_issue_label`.
- **Close**: `save_comment` with the reason, then `save_issue` with `state: "Done"` (resolved) or `state: "Canceled"` (won't do).

Issue identifiers look like `GABE-42`. A bare `#42` in this repo (commit messages, code comments, ADRs) means GitHub issue or PR #42 from before the move to Linear.

## When a skill says "publish to the issue tracker"

Create a Linear issue in team `GABE`, project `Yogurt`.

## When a skill says "fetch the relevant ticket"

`get_issue` by identifier, then `list_comments`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes / Decisions-so-far / Fog body.
- **Child ticket**: an issue created with `parentId` set to the map (Linear sub-issue). Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev.
- **Blocking**: Linear's native relation: `save_issue` with `id` and `blockedBy: ["GABE-12"]` (remove with `removeBlockedBy`). A ticket is unblocked when every blocker is `Done` or `Canceled`.
- **Frontier query**: `list_issues` with `parentId` set to the map, open states only; `get_issue` with `includeRelations: true` on each; drop any with an open blocker or an assignee; first in map order wins.
- **Claim**: `save_issue` with `assignee: "me"` and `state: "In Progress"`: the session's first write.
- **Resolve**: `save_comment` with the answer, `save_issue` with `state: "Done"`, then append a context pointer (gist + link) to the map's Decisions-so-far.
