#!/usr/bin/env python3
"""Initialize a new skill directory with SKILL.md template and optional resources."""

import argparse
import os
import sys
import re

SKILL_TEMPLATE = """---
name: {name}
description: "TODO: Describe what this skill does and when to use it. Be specific about triggers."
# arguments: []
# argument-hint: ""
# context: inline
# agent_type: ""
# allowed_tools: []
# user-invocable: true
---

# {title}

TODO: Write instructions for using this skill.

## Overview

TODO: Describe the skill's purpose and core workflow.
"""

SKILL_TEMPLATE_WITH_SCRIPTS = """---
name: {name}
description: "TODO: Describe what this skill does and when to use it. Be specific about triggers."
# arguments: []
# argument-hint: ""
# context: inline
# agent_type: ""
# allowed_tools: []
# user-invocable: true
---

# {title}

TODO: Write instructions for using this skill.

## Overview

TODO: Describe the skill's purpose and core workflow.

## Scripts

The following scripts are available in the `scripts/` directory:

- TODO: List scripts and their purposes
"""

SKILL_TEMPLATE_WITH_REFERENCES = """---
name: {name}
description: "TODO: Describe what this skill does and when to use it. Be specific about triggers."
# arguments: []
# argument-hint: ""
# context: inline
# agent_type: ""
# allowed_tools: []
# user-invocable: true
---

# {title}

TODO: Write instructions for using this skill.

## Overview

TODO: Describe the skill's purpose and core workflow.

## References

For detailed information, see:

- TODO: List reference files and when to read them
"""

SKILL_TEMPLATE_FULL = """---
name: {name}
description: "TODO: Describe what this skill does and when to use it. Be specific about triggers."
# arguments: []
# argument-hint: ""
# context: inline
# agent_type: ""
# allowed_tools: []
# user-invocable: true
---

# {title}

TODO: Write instructions for using this skill.

## Overview

TODO: Describe the skill's purpose and core workflow.

## Scripts

The following scripts are available in the `scripts/` directory:

- TODO: List scripts and their purposes

## References

For detailed information, see:

- TODO: List reference files and when to read them

## Assets

Templates and output files are in the `assets/` directory:

- TODO: List assets and their purposes
"""

EXAMPLE_SCRIPT = """#!/usr/bin/env python3
\"\"\"TODO: Describe what this script does.\"\"\"

import sys


def main():
    # TODO: Implement script logic
    print("Hello from {name} skill!")
    return 0


if __name__ == "__main__":
    sys.exit(main())
"""

EXAMPLE_REFERENCE = """# {title} Reference

TODO: Add detailed reference material here.

This file is loaded by the agent on demand — only when the information is needed.
Keep it focused on a single topic or domain.
"""

NAME_PATTERN = re.compile(r'^[a-z0-9]+(-[a-z0-9]+)*$')
MAX_NAME_LENGTH = 64


def validate_name(name: str) -> str | None:
    if not name:
        return "name is required"
    if len(name) > MAX_NAME_LENGTH:
        return f"name exceeds {MAX_NAME_LENGTH} characters"
    if not NAME_PATTERN.match(name):
        return "name must be lowercase alphanumeric with hyphens (e.g., 'my-skill')"
    return None


def name_to_title(name: str) -> str:
    return name.replace("-", " ").title()


def main():
    parser = argparse.ArgumentParser(
        description="Initialize a new skill directory"
    )
    parser.add_argument("name", help="Skill name (lowercase, hyphens only)")
    parser.add_argument(
        "--path", required=True,
        help="Parent directory where skill folder will be created"
    )
    parser.add_argument(
        "--resources", default="",
        help="Comma-separated resource dirs to create: scripts,references,assets"
    )
    parser.add_argument(
        "--examples", action="store_true",
        help="Add example placeholder files in resource directories"
    )
    parser.add_argument(
        "--context", choices=["inline", "fork", "spawn"], default="inline",
        help="Execution context for the skill (default: inline)"
    )
    parser.add_argument(
        "--agent-type", default="",
        help="Agent type to use when context is fork or spawn"
    )
    parser.add_argument(
        "--arguments", default="",
        help="Comma-separated argument names for the skill"
    )

    args = parser.parse_args()

    # Validate name
    err = validate_name(args.name)
    if err:
        print(f"Error: {err}", file=sys.stderr)
        return 1

    # Parse resources
    resources = set()
    if args.resources:
        for r in args.resources.split(","):
            r = r.strip().lower()
            if r in ("scripts", "references", "assets"):
                resources.add(r)
            elif r:
                print(f"Warning: unknown resource type '{r}', skipping", file=sys.stderr)

    # Build skill directory path
    skill_dir = os.path.join(args.path, args.name)

    if os.path.exists(skill_dir):
        print(f"Error: directory already exists: {skill_dir}", file=sys.stderr)
        return 1

    # Create skill directory
    os.makedirs(skill_dir, exist_ok=True)

    title = name_to_title(args.name)

    # Select template based on resources
    has_scripts = "scripts" in resources
    has_refs = "references" in resources
    has_assets = "assets" in resources

    if has_scripts and (has_refs or has_assets):
        template = SKILL_TEMPLATE_FULL
    elif has_scripts:
        template = SKILL_TEMPLATE_WITH_SCRIPTS
    elif has_refs:
        template = SKILL_TEMPLATE_WITH_REFERENCES
    else:
        template = SKILL_TEMPLATE

    # Write SKILL.md
    skill_md = template.format(name=args.name, title=title)

    # Uncomment and populate context if non-default value provided
    if args.context and args.context != "inline":
        skill_md = skill_md.replace(
            "# context: inline",
            f"context: {args.context}"
        )

    # Uncomment and populate agent_type if provided
    if args.agent_type:
        skill_md = skill_md.replace(
            '# agent_type: ""',
            f'agent_type: "{args.agent_type}"'
        )

    # Uncomment and populate arguments if provided
    if args.arguments:
        arg_list = [a.strip() for a in args.arguments.split(",") if a.strip()]
        args_yaml = "[" + ", ".join(arg_list) + "]"
        skill_md = skill_md.replace(
            "# arguments: []",
            f"arguments: {args_yaml}"
        )

    skill_md_path = os.path.join(skill_dir, "SKILL.md")
    with open(skill_md_path, "w") as f:
        f.write(skill_md)

    # Create resource directories
    for resource in sorted(resources):
        resource_dir = os.path.join(skill_dir, resource)
        os.makedirs(resource_dir, exist_ok=True)

        if args.examples:
            if resource == "scripts":
                example_path = os.path.join(resource_dir, f"example.py")
                with open(example_path, "w") as f:
                    f.write(EXAMPLE_SCRIPT.format(name=args.name))
                os.chmod(example_path, 0o755)

            elif resource == "references":
                example_path = os.path.join(resource_dir, "example.md")
                with open(example_path, "w") as f:
                    f.write(EXAMPLE_REFERENCE.format(title=title))

            elif resource == "assets":
                # Just create a .gitkeep so the directory is tracked
                gitkeep = os.path.join(resource_dir, ".gitkeep")
                with open(gitkeep, "w") as f:
                    pass

    print(f"Created skill '{args.name}' at {skill_dir}")
    print(f"  SKILL.md: {skill_md_path}")
    for resource in sorted(resources):
        print(f"  {resource}/: {os.path.join(skill_dir, resource)}")

    print(f"\nNext steps:")
    print(f"  1. Edit {skill_md_path} — fill in description and instructions")
    if resources:
        print(f"  2. Implement resource files in {', '.join(sorted(resources))}/")
    print(f"  3. Test the skill by activating it with /use {args.name}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
