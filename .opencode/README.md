# evm-api agents

Project-local OpenCode subagents for Go/backend work. Start a new session or
reload agent discovery after editing these files. The primary agent owns scope,
requirements, delegation decisions, acceptance, and final verification; these
roles are leaves and do not launch other agents. Read `AGENTS.md` and the
applicable `.agents/skills/` guidance before using a role. Preserve unrelated
worktree changes. A source change is not permission to deploy or call live
providers.

| Role | Use |
| --- | --- |
| `scout` | Read-only discovery of Go flows and repository contracts |
| `architecture`, `architecture-review` | Proposal and independent design/conformance review for complex changes |
| `implementor-junior`, `implementor-middle` | Narrow or coupled implementation within evm-api |
| `implementor-senior`, `implementor-very-senior` | Complex or authorized cross-repository changes following independent design acceptance |
| `check`, `devops-check` | Go test/lint and delivery configuration verification |
| `review`, `review-fast`, `review-deep`, `review-skeptic`, `review-consensus` | Independent source review and finding reconciliation |
| `test-value-review` | Check regression value of added tests |
| `fix-review`, `fix-architect`, `fixer`, `verify` | Triage, approve, apply, and verify review remediation |
| `spec-implementer`, `spec-verifier` | Author and independently check OpenSpec artifacts |

Use `./make.sh test` for normal validation; focus with `unit`, `http`, or
`cloudrun` and `-run` as appropriate. Integration suites share a database and
port, so never run them in parallel. Checks requiring unavailable services or
credentials are **BLOCKED**, not passed. For senior work request an independent
`architecture-review` Accept before implementation and conformance review after
checks; for other work use only the roles warranted by the task. These roles
inherit the selected model and project permissions unless overridden locally.
