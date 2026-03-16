---
name: Implementation Task
about: AI-consumable task with full context and acceptance criteria
title: ''
labels: 'task,backlog'
assignees: ''
---

## Problem

<!-- What is broken or missing? -->

## Spec

<!-- Exact interface, behavior, function signatures -->

## Out of Scope

<!-- Explicitly list what this issue must NOT change -->

## Files to Modify

<!-- List of files with brief description of changes -->

## Test Plan

<!-- Expected inputs/outputs, test fixtures, how to verify -->

## Acceptance Criteria

- [ ] Behavior matches Spec exactly
- [ ] Existing tests pass
- [ ] New tests cover the change
- [ ] No regression in related CLI features

## Automation Policy

<!-- Guardrails for PM/Dev automation loop -->
- Max automated attempts: 3
- If same failure repeats 2+ times, add `needs-human` and stop auto-retry
- Do not auto-merge without Claude approval + CI green

## References

<!-- Links to design docs, related issues, code snippets -->
