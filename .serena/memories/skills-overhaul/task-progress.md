# Skills Overhaul Worktree — Task Progress

Worktree: `/home/eugen-dev/Workflows/picoclaw/.worktrees/skills-overhaul`
Branch: `feature/skills-overhaul`

## Task Status

- [x] Task 1: Extend SkillMetadata — DONE (commit b6dd8b7)
  - Extended SkillMetadata struct: Arguments, ArgumentHint, Context, AgentType, AllowedTools, UserInvocable
  - Added rawSkillMeta intermediate struct with *bool UserInvocable for nil-detection
  - applyDefaults() applies after all parse paths (JSON, YAML, no-frontmatter)
  - Context validated against inline/fork/spawn, defaults to "inline"; invalid values fall back to "inline"
  - UserInvocable defaults to true when field absent; explicit false honored
  - Created pkg/skills/metadata_test.go — 3 tests (NewFields, Defaults, InvalidContext), all passing
  - All 75 pkg/skills tests pass
- [x] Task 2: Argument Substitution — DONE (commit 210f157)
  - Created `pkg/skills/substitute.go` — SubstituteArgs() replacing ${ARGUMENTS}, ${ARG1}..${ARGN}, ${SKILL_DIR}, ${WORKSPACE}
  - Created `pkg/skills/substitute_test.go` — 5 tests, all passing
  - Undefined positional args left as-is (no silent erasure)
- [x] Task 3: Update Skills Summary — DONE (commit 02f919b)
  - Extended SkillInfo struct with ArgumentHint, Context, UserInvocable fields
  - ListSkills() now populates those fields from parsed SkillMetadata
  - BuildSkillsSummary() emits `<usage>/name arg-hint</usage>` when ArgumentHint is present
  - Added public GetSkillMetadata(name) and GetSkillDir(name) methods on SkillsLoader
  - All pkg/skills tests pass
- [ ] Task 4: use_skill Tool — pending (task #61)
- [ ] Task 5: Register use_skill Tool — pending (task #62)
- [ ] Task 6: Slash Command Handler — pending (task #63)
- [ ] Task 7: Skill-Creator Rewrite — pending (task #64)
- [ ] Tasks 8-9: Update init/package scripts — pending (task #65)
