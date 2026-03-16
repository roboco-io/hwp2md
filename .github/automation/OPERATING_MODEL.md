# PM/Dev Automation Operating Model

## Roles
- Claude PM: issue definition, review decision, prioritization.
- Codex Dev: implementation, tests, PR delivery.

## State Machine
- `backlog` -> `ready-codex` -> `in-progress` -> `needs-claude-review` -> `approved` -> `done`
- Exception states: `blocked`, `needs-human`

## Automation Jobs
- `automation-label-bootstrap.yml`: ensures required labels exist.
- `automation-state-machine.yml`:
  - issue enters execution when labeled `ready-codex`
  - PRs are labeled `needs-claude-review`
  - merged PRs are labeled `done`
  - optional follow-up issue generation when PR has `auto-loop`
- `claude-review-scheduler.yml`:
  - reminds pending Claude review every 6h (max 3 reminders)
  - escalates to `needs-human` + `blocked` after max reminders

## Loop Prevention Rules
- Max automated issue attempts: 3
- Max review reminders: 3
- Escalate to `needs-human` when thresholds are exceeded
- Follow-up issue creation is opt-in via `auto-loop` label only

## Merge Gate
- Required: CI green + Claude approval
- Do not auto-merge when PR is `blocked` or `needs-human`
