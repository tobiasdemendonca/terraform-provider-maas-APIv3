# 0001. Record architecture decisions

Status: draft (2026-07-07)

## Context

Agents and people working on this repository need to understand why things are the way they are, not just what they are. That history lives nowhere if decisions are only visible as code.

## Decision

We record architecture decisions as ADRs in `docs/decisions/`, numbered `NNNN-title.md`, using the [Michael Nygard template](https://github.com/architecture-decision-record/architecture-decision-record/tree/main/locales/en/templates/decision-record-template-by-michael-nygard): Title, Status, Context, Decision, Consequences. See [what ADRs are and how to start](https://github.com/architecture-decision-record/architecture-decision-record#how-to-start-using-adrs).

Write an ADR for general design principles that apply across the codebase. Do not write one for anything specific to a single resource; solve that with expressive code or a comment.

## Consequences

- The reasoning behind repository-wide conventions survives refactors and personnel changes, and is loadable context for agents.
- Single-resource decisions stay next to the code they describe.
