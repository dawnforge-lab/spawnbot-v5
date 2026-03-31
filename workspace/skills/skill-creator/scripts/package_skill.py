#!/usr/bin/env python3
"""Validate and package a skill into a distributable .skill file."""

import argparse
import os
import re
import sys
import zipfile

NAME_PATTERN = re.compile(r'^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$')
MAX_NAME_LENGTH = 64
MAX_DESCRIPTION_LENGTH = 1024
ALLOWED_RESOURCE_DIRS = {"scripts", "references", "assets"}
FORBIDDEN_FILES = {
    "README.md", "INSTALLATION_GUIDE.md", "QUICK_REFERENCE.md",
    "CHANGELOG.md", "LICENSE", "LICENSE.md",
}


class ValidationError:
    def __init__(self, severity: str, message: str):
        self.severity = severity  # "error" or "warning"
        self.message = message

    def __str__(self):
        return f"[{self.severity.upper()}] {self.message}"


def parse_frontmatter(content: str) -> tuple[dict[str, str] | None, str]:
    """Parse YAML frontmatter from SKILL.md content.
    Returns (frontmatter_dict, body) or (None, content) if no frontmatter.
    """
    lines = content.split("\n")
    if not lines or lines[0].strip() != "---":
        return None, content

    end = -1
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            end = i
            break

    if end == -1:
        return None, content

    frontmatter_text = "\n".join(lines[1:end])
    body = "\n".join(lines[end + 1:]).lstrip("\n")

    # Simple YAML parsing for flat key-value pairs
    result = {}
    for line in frontmatter_text.split("\n"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if ":" in line:
            key, _, value = line.partition(":")
            key = key.strip()
            value = value.strip().strip('"').strip("'")
            result[key] = value

    return result, body


def validate_skill(skill_dir: str) -> list[ValidationError]:
    """Validate a skill directory. Returns list of errors/warnings."""
    errors = []

    # Check directory exists
    if not os.path.isdir(skill_dir):
        errors.append(ValidationError("error", f"Directory does not exist: {skill_dir}"))
        return errors

    # Check SKILL.md exists
    skill_md_path = os.path.join(skill_dir, "SKILL.md")
    if not os.path.isfile(skill_md_path):
        errors.append(ValidationError("error", "SKILL.md not found"))
        return errors

    # Read and parse SKILL.md
    with open(skill_md_path, "r") as f:
        content = f.read()

    frontmatter, body = parse_frontmatter(content)

    # Validate frontmatter exists
    if frontmatter is None:
        errors.append(ValidationError("error", "SKILL.md missing YAML frontmatter (---/--- block)"))
        return errors

    # Validate name
    name = frontmatter.get("name", "")
    if not name:
        errors.append(ValidationError("error", "Frontmatter missing 'name' field"))
    elif len(name) > MAX_NAME_LENGTH:
        errors.append(ValidationError("error", f"Name exceeds {MAX_NAME_LENGTH} characters: '{name}'"))
    elif not NAME_PATTERN.match(name):
        errors.append(ValidationError("error", f"Invalid name format: '{name}' (must be alphanumeric with hyphens)"))

    # Validate directory name matches skill name
    dir_name = os.path.basename(os.path.normpath(skill_dir))
    if name and dir_name != name:
        errors.append(ValidationError("warning", f"Directory name '{dir_name}' differs from skill name '{name}'"))

    # Validate description
    description = frontmatter.get("description", "")
    if not description:
        errors.append(ValidationError("error", "Frontmatter missing 'description' field"))
    elif description.startswith("TODO"):
        errors.append(ValidationError("error", "Description is still a TODO placeholder"))
    elif len(description) > MAX_DESCRIPTION_LENGTH:
        errors.append(ValidationError("error", f"Description exceeds {MAX_DESCRIPTION_LENGTH} characters"))
    elif len(description) < 20:
        errors.append(ValidationError("warning", "Description is very short — include trigger context for better discoverability"))

    # Validate new fields
    context = frontmatter.get('context', 'inline')
    if context not in ('inline', 'fork', 'spawn'):
        errors.append(ValidationError(
            'error', f'Invalid context value: {context}. Must be inline, fork, or spawn.'))

    agent_type = frontmatter.get('agent_type', '')
    if agent_type and not re.match(NAME_PATTERN, agent_type):
        errors.append(ValidationError(
            'error', f'Invalid agent_type: {agent_type}. Must match {NAME_PATTERN}.'))

    arguments = frontmatter.get('arguments', [])
    if arguments and not isinstance(arguments, list):
        errors.append(ValidationError(
            'error', 'arguments must be a list of strings.'))

    if context in ('fork', 'spawn') and not agent_type:
        errors.append(ValidationError(
            'warning', f'context is {context} but no agent_type specified. Will use default agent.'))

    # Check for extra frontmatter fields
    known_fields = {"name", "description", "metadata", "arguments", "argument-hint", "context", "agent_type", "allowed_tools", "user-invocable"}
    extra_fields = set(frontmatter.keys()) - known_fields
    if extra_fields:
        errors.append(ValidationError("warning", f"Unknown frontmatter fields: {', '.join(sorted(extra_fields))}"))

    # Validate body is not empty/placeholder
    body_stripped = body.strip()
    if not body_stripped:
        errors.append(ValidationError("error", "SKILL.md body is empty"))
    elif body_stripped.startswith("TODO"):
        errors.append(ValidationError("warning", "SKILL.md body starts with TODO — ensure instructions are complete"))

    # Check body length
    body_lines = body_stripped.split("\n")
    if len(body_lines) > 500:
        errors.append(ValidationError("warning", f"SKILL.md body is {len(body_lines)} lines — consider splitting into references"))

    # Check for forbidden files
    for item in os.listdir(skill_dir):
        if item in FORBIDDEN_FILES:
            errors.append(ValidationError("warning", f"Unnecessary file '{item}' — skills should only contain essential files"))

    # Check subdirectories
    for item in os.listdir(skill_dir):
        item_path = os.path.join(skill_dir, item)
        if os.path.isdir(item_path) and item not in ALLOWED_RESOURCE_DIRS:
            if not item.startswith("."):
                errors.append(ValidationError("warning", f"Unknown directory '{item}/' — expected: scripts/, references/, assets/"))

    # Check that referenced files exist
    for ref_pattern in [r'\[([^\]]+)\]\(([^)]+)\)', r'`([^`]*\.(?:py|sh|md|txt))`']:
        for match in re.finditer(ref_pattern, body):
            ref_path = match.group(2) if '(' in match.group(0) else match.group(1)
            # Skip URLs and anchors
            if ref_path.startswith(("http://", "https://", "#")):
                continue
            full_path = os.path.join(skill_dir, ref_path)
            if not os.path.exists(full_path):
                errors.append(ValidationError("warning", f"Referenced file not found: {ref_path}"))

    # Check scripts are executable
    scripts_dir = os.path.join(skill_dir, "scripts")
    if os.path.isdir(scripts_dir):
        for script in os.listdir(scripts_dir):
            script_path = os.path.join(scripts_dir, script)
            if os.path.isfile(script_path) and not os.access(script_path, os.X_OK):
                errors.append(ValidationError("warning", f"Script not executable: scripts/{script}"))

    return errors


def package_skill(skill_dir: str, output_dir: str | None = None) -> str:
    """Package skill into a .skill zip file. Returns output path."""
    skill_name = os.path.basename(os.path.normpath(skill_dir))
    if output_dir is None:
        output_dir = os.path.dirname(os.path.abspath(skill_dir))

    os.makedirs(output_dir, exist_ok=True)
    output_path = os.path.join(output_dir, f"{skill_name}.skill")

    with zipfile.ZipFile(output_path, "w", zipfile.ZIP_DEFLATED) as zf:
        for root, dirs, files in os.walk(skill_dir):
            # Skip hidden directories
            dirs[:] = [d for d in dirs if not d.startswith(".")]
            for file in files:
                if file.startswith("."):
                    continue
                file_path = os.path.join(root, file)
                arcname = os.path.join(
                    skill_name,
                    os.path.relpath(file_path, skill_dir)
                )
                zf.write(file_path, arcname)

    return output_path


def main():
    parser = argparse.ArgumentParser(
        description="Validate and package a skill for distribution"
    )
    parser.add_argument("skill_dir", help="Path to the skill directory")
    parser.add_argument("output_dir", nargs="?", help="Output directory for .skill file (default: parent of skill dir)")
    parser.add_argument("--validate-only", action="store_true", help="Only validate, don't package")

    args = parser.parse_args()

    skill_dir = os.path.abspath(args.skill_dir)

    # Validate
    print(f"Validating skill at: {skill_dir}")
    errors = validate_skill(skill_dir)

    has_errors = any(e.severity == "error" for e in errors)
    has_warnings = any(e.severity == "warning" for e in errors)

    if errors:
        for e in errors:
            print(f"  {e}")
        print()

    if has_errors:
        print("Validation FAILED. Fix errors before packaging.")
        return 1

    if has_warnings:
        print("Validation passed with warnings.")
    else:
        print("Validation passed.")

    if args.validate_only:
        return 0

    # Package
    print()
    output_path = package_skill(skill_dir, args.output_dir)
    print(f"Packaged skill to: {output_path}")

    # Show contents
    with zipfile.ZipFile(output_path, "r") as zf:
        print(f"\nContents ({len(zf.namelist())} files):")
        for name in sorted(zf.namelist()):
            info = zf.getinfo(name)
            print(f"  {name} ({info.file_size} bytes)")

    return 0


if __name__ == "__main__":
    sys.exit(main())
