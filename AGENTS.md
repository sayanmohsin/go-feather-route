# Go Feather Route agent instructions

Read [the master plan](docs/master-plan.md) before planning or implementing work.
It defines product direction, phase gates, evidence requirements, and handoff rules.
API documentation and tests remain the contract for currently available behavior.

- Inspect Git status first; preserve unrelated changes and stage explicit paths.
- Work in Go Feather Route only unless the user authorizes another repository.
- Develop a general-purpose, lightweight Go AI gateway and eventual LiteLLM
  alternative. Do not treat the master plan as authorization to migrate applications.
- Choose the earliest unmet prerequisite in the plan; verify existing evidence
  before repeating work or marking a capability complete.
- Measure the baseline before optimization. Preserve correctness and compatibility;
  do not claim lower latency or memory from implementation language alone.
- Keep caching, retrieval, MCP, and provider-specific extensions optional.
- Use small Go packages, explicit ownership, bounded work, and minimal dependencies.
- Update affected API/config documentation and tests with implementation changes.
- Run the phase checks and leave a reproducible handoff including remaining gaps.
- Commit, push, publish, and run application experiments only when authorized.
  Never bypass verification hooks or overwrite release tags.

## Nice Code

Nice Code is an advisory review layer alongside the Go toolchain, not a
replacement for `gofmt`, `go vet`, staticcheck, security checks, or tests. Run
`make nice-code` for changed files and `make nice-code-all` for an explicit full
scan. The project skill lock is intentionally empty because the current Nice
Code registry has no Go-specific skill; use `nice-code skills list` when new
relevant skills are published and add only reviewed, applicable entries.
