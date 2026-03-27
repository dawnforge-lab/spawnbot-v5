# Output Skill Patterns

Reference for designing skills that produce specific output formats, maintain quality standards, or follow templates.

## Template-Based Output

For skills that generate files from a known structure:

```markdown
## Output Format

Use the template in `assets/template.html` as the base. Customize:

1. Replace `TITLE_PLACEHOLDER` with the document title
2. Replace `CONTENT_PLACEHOLDER` with generated content
3. Keep the CSS and structure intact

## Example

Input: "Create a report for Q1 sales"
Output: HTML file using the template with sales data populated
```

**When to use:** Document generation, email drafting, report creation.

**Key principle:** Ship the template as an asset. Don't describe the template in prose — the agent will re-invent it each time.

## Format Specification Pattern

For skills that must produce precise output formats:

```markdown
## Output Requirements

The output must be valid JSON matching this schema:

```json
{
  "type": "object",
  "required": ["id", "status", "results"],
  "properties": {
    "id": { "type": "string" },
    "status": { "type": "string", "enum": ["pass", "fail"] },
    "results": { "type": "array", "items": { ... } }
  }
}
```

## Validation

After generating output, verify:
1. JSON is valid (parse it)
2. Required fields are present
3. Enum values are correct
```

**When to use:** API responses, configuration files, data exports.

**Key principle:** Give the agent a schema or concrete example, not a prose description. "The output should have an id field" is vague. A JSON schema is unambiguous.

## Quality Checklist Pattern

For skills where output quality varies and needs consistent standards:

```markdown
## Quality Criteria

Before delivering output, verify:

- [ ] Accurate: All facts are verifiable
- [ ] Complete: All requested sections included
- [ ] Concise: No filler or unnecessary repetition
- [ ] Formatted: Follows the specified format exactly
- [ ] Tested: Code examples actually run (if applicable)
```

**When to use:** Content creation, documentation, code generation.

**Key principle:** Make criteria binary (pass/fail), not subjective. "Is it good?" is useless. "Does it compile?" is actionable.

## Multi-Format Output Pattern

For skills that produce different output types based on context:

```markdown
## Output Formats

### Markdown (default)
Use for documentation and notes. Follow standard Markdown conventions.

### HTML
Use when the user needs a rendered document. Use the template in `assets/base.html`.

### PDF
Use when the user needs a printable document. Generate HTML first, then convert:
```bash
scripts/html_to_pdf.py <input.html> <output.pdf>
```

## Format Selection

If the user doesn't specify, default to Markdown. If they mention "printable", "share", or "presentation", use PDF.
```

**When to use:** Skills that serve multiple output channels.

**Key principle:** Define clear selection heuristics. The agent shouldn't ask "what format?" every time if the context makes it obvious.

## Example-Driven Pattern

For skills where showing is more effective than telling:

```markdown
## Examples

### Simple case
**Input:** "Summarize this article"
**Output:**
> The article discusses three key trends in renewable energy...

### Complex case
**Input:** "Summarize this article with key quotes and data points"
**Output:**
> ## Summary
> The article discusses...
>
> ## Key Quotes
> - "Renewable energy capacity grew 50% in 2025" (para 3)
>
> ## Data Points
> - Solar: 450GW installed
> - Wind: 320GW installed
```

**When to use:** When the output format is hard to describe but easy to demonstrate.

**Key principle:** Show 2-3 examples at increasing complexity. The agent generalizes from examples better than from rules.

## Anti-Patterns

**Vague quality standards:** "Make it professional" means different things in different contexts. Be specific about what professional means for this skill.

**Missing examples:** A format specification without examples forces the agent to guess. Always include at least one complete example.

**Over-specified formatting:** Don't dictate every line break and spacing choice. Specify structure and let the agent handle presentation.

**No validation step:** If the output format matters, include a way to verify it. The agent can check its own work if you tell it how.
