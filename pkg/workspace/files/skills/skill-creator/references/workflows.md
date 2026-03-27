# Workflow Skill Patterns

Reference for designing skills with multi-step processes, conditional logic, and sequential workflows.

## Sequential Workflow Pattern

For skills where steps must be followed in order:

```markdown
## Process

### Step 1: Gather inputs
Collect X, Y, Z from the user or context.

### Step 2: Validate
Check that inputs meet requirements:
- Condition A
- Condition B

### Step 3: Execute
Run the operation using the validated inputs.

### Step 4: Verify
Confirm the output matches expectations.
```

**When to use:** Deployment pipelines, data migrations, multi-stage transformations.

**Key principle:** Each step should have clear entry/exit criteria. The agent needs to know when a step is complete before moving on.

## Conditional Branching Pattern

For skills where the workflow changes based on context:

```markdown
## Determine approach

First, identify which case applies:

### Case A: [condition]
Follow these steps...

### Case B: [condition]
Follow these alternative steps...

### Common: Final steps
Regardless of case, finish with...
```

**When to use:** Skills that handle multiple input types, platforms, or configurations.

**Key principle:** Put the branching decision early. Don't make the agent read 200 lines before discovering which path applies.

## Iterative Refinement Pattern

For skills where the output improves through cycles:

```markdown
## Process

1. Generate initial output
2. Evaluate against criteria:
   - Criterion A: [specific check]
   - Criterion B: [specific check]
3. If criteria not met, adjust and repeat from step 1
4. Maximum 3 iterations — if still not passing, report what's failing
```

**When to use:** Code generation, content creation, optimization tasks.

**Key principle:** Define concrete exit criteria. Without them, the agent loops indefinitely or stops arbitrarily.

## Checkpoint Pattern

For long workflows where partial progress should be preserved:

```markdown
## Process

### Phase 1: Setup
... steps ...
**Checkpoint:** Confirm setup is complete before proceeding.

### Phase 2: Core work
... steps ...
**Checkpoint:** Verify core output before finishing.

### Phase 3: Cleanup
... steps ...
```

**When to use:** Workflows that can fail midway and need recovery points.

**Key principle:** Each checkpoint should produce visible confirmation so the user knows where things stand.

## Script-Assisted Workflow

For workflows with both deterministic and creative steps:

```markdown
## Process

1. Run `scripts/analyze.py <input>` to assess the situation
2. Based on the analysis output, decide the approach:
   - If metric > threshold: use aggressive optimization
   - Otherwise: use conservative approach
3. Implement the chosen approach
4. Run `scripts/validate.py <output>` to verify
```

**When to use:** When part of the workflow is mechanical (scripts) and part requires judgment (agent reasoning).

**Key principle:** Scripts handle the fragile/deterministic parts. The agent handles the creative/contextual parts. Don't try to script judgment calls.

## Anti-Patterns

**Over-scripting:** Turning every step into a script removes the agent's ability to adapt. Only script the parts that must be exact.

**Missing error paths:** Every "run X" step should say what to do if X fails. The agent needs to know whether to retry, skip, or stop.

**Implicit ordering:** If steps must happen in order, say so explicitly. "Do A, B, C" is ambiguous — "Do A, then B (requires A's output), then C" is clear.

**Unbounded loops:** "Keep trying until it works" will burn tokens. Always set a maximum iteration count.
