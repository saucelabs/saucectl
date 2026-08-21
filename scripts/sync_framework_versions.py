#!/usr/bin/env python3

from __future__ import annotations

import json
import os
import re
import sys
import urllib.request
from dataclasses import dataclass, field

RAW = "https://raw.githubusercontent.com/saucelabs"

# Repo root of the checked-out saucectl repo (the workflow runs from there).
REPO_ROOT = os.environ.get("GITHUB_WORKSPACE", ".")


@dataclass
class Framework:
    name: str
    runner_repo: str           # public runner repo under github.com/saucelabs
    # ordered candidate keys in the runner package.json dependencies/devDeps
    version_keys: list[str]
    # schema files in saucectl that carry this framework's version enum
    schema_files: list[str]


FRAMEWORKS = [
    Framework(
        name="cypress",
        runner_repo="sauce-cypress-runner",
        version_keys=["cypress"],
        schema_files=[
            "api/v1/framework/cypress.schema.json",
            "api/saucectl.schema.json",
        ],
    ),
    Framework(
        name="playwright",
        runner_repo="sauce-playwright-runner",
        version_keys=["playwright", "@playwright/test"],
        schema_files=[
            "api/v1alpha/framework/playwright.schema.json",
            "api/saucectl.schema.json",
        ],
    ),
    Framework(
        name="testcafe",
        runner_repo="sauce-testcafe-runner",
        version_keys=["testcafe"],
        schema_files=[
            "api/v1alpha/framework/testcafe.schema.json",
            "api/saucectl.schema.json",
        ],
    ),
]

# A line inside a version enum, e.g.  `        "15.14.2",`
VERSION_LINE = re.compile(r'^(\s*)"(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.\-]+)?)"(,?)\s*$')


# --------------------------------------------------------------------------- #
# Version helpers
# --------------------------------------------------------------------------- #
def version_key(v: str):
    """Sort key: (major, minor, patch, is_release, prerelease).

    A release sorts above a prerelease of the same x.y.z. Build metadata is
    ignored for ordering (per semver).
    """
    core = re.split(r"[-+]", v)[0]
    pre = ""
    m = re.match(r"^\d+\.\d+\.\d+-([0-9A-Za-z.\-]+)", v)
    if m:
        pre = m.group(1)
    parts = [int(p) for p in core.split(".")]
    while len(parts) < 3:
        parts.append(0)
    # is_release = 1 (release) sorts above 0 (prerelease)
    return (parts[0], parts[1], parts[2], 0 if pre else 1, pre)


def is_newer(candidate: str, current_top: str) -> bool:
    return version_key(candidate) > version_key(current_top)


# --------------------------------------------------------------------------- #
# Reading runner versions
# --------------------------------------------------------------------------- #
def fetch_runner_version(fw: Framework, branch: str = "main") -> str:
    url = f"{RAW}/{fw.runner_repo}/{branch}/package.json"
    with urllib.request.urlopen(url, timeout=30) as r:
        pkg = json.load(r)
    deps = {**pkg.get("dependencies", {}), **pkg.get("devDependencies", {})}
    for key in fw.version_keys:
        if key in deps:
            # strip range specifiers like ^ ~ >=
            return re.sub(r"^[\^~>=<\s]*", "", deps[key]).strip()
    raise KeyError(
        f"none of {fw.version_keys} found in {fw.runner_repo}/package.json"
    )


# --------------------------------------------------------------------------- #
# Locating + editing enums
# --------------------------------------------------------------------------- #
def find_version_enums(obj, path=""):
    """Yield (json_path, enum_list) for every enum that holds version strings."""
    out = []
    if isinstance(obj, dict):
        enum = obj.get("enum")
        if isinstance(enum, list) and any(
            isinstance(v, str) and re.match(r"^\d+\.\d+", v) for v in enum
        ):
            out.append((path, enum))
        for k, v in obj.items():
            out += find_version_enums(v, f"{path}.{k}")
    elif isinstance(obj, list):
        for i, v in enumerate(obj):
            out += find_version_enums(v, f"{path}[{i}]")
    return out


def enums_for_framework(data, fw_name):
    """Return only the enums that belong to <fw_name>'s version property."""
    needle = f".{fw_name}.properties.version"
    return [
        (p, e)
        for p, e in find_version_enums(data)
        if needle in p
    ]


def edit_enum_text(lines: list[str], enum: list[str], new_version: str) -> bool:
    """Apply the rolling-window edit to `lines` in place.

    enum[0] is always "package.json"; enum[1] is the current newest; enum[-1]
    the oldest. We:
      - insert `new_version` as a version line immediately before the enum[1]
        line, copying its indentation;
      - delete the enum[-1] line (the oldest, which has no trailing comma);
      - strip the trailing comma from the enum[-2] line (now the last element).

    Anchors on the exact version strings, which are unique per file. Raises if an
    anchor is missing or ambiguous, so the workflow fails loudly rather than
    writing a corrupt file.
    """
    current_top = enum[1]
    oldest = enum[-1]
    second_oldest = enum[-2]

    def find_unique(value: str) -> int:
        idxs = [
            i for i, ln in enumerate(lines)
            if (m := VERSION_LINE.match(ln)) and m.group(2) == value
        ]
        if len(idxs) != 1:
            raise RuntimeError(
                f'expected exactly one version line "{value}", found {len(idxs)}'
            )
        return idxs[0]

    top_idx = find_unique(current_top)
    indent = VERSION_LINE.match(lines[top_idx]).group(1)

    # 1) insert the new version above the current top
    lines.insert(top_idx, f'{indent}"{new_version}",\n')

    # indices below the insertion shifted by +1
    oldest_idx = find_unique(oldest)
    second_idx = find_unique(second_oldest)

    # 2) remove the oldest line
    del lines[oldest_idx]

    # 3) ensure the new last element has no trailing comma
    m = VERSION_LINE.match(lines[second_idx if second_idx < oldest_idx else second_idx - 1])
    li = second_idx if second_idx < oldest_idx else second_idx - 1
    lines[li] = f'{indent}"{second_oldest}"\n'
    return True


# --------------------------------------------------------------------------- #
# Main
# --------------------------------------------------------------------------- #
@dataclass
class Change:
    framework: str
    old: str
    new: str
    dropped: str
    files: list[str] = field(default_factory=list)


def run() -> int:
    changes: list[Change] = []
    notes: list[str] = []

    for fw in FRAMEWORKS:
        try:
            runner_version = fetch_runner_version(fw)
        except Exception as e:  # noqa: BLE001
            notes.append(f"⚠️  {fw.name}: could not read runner version ({e})")
            continue

        edited_files: list[str] = []
        old_top = None
        dropped = None

        for rel in fw.schema_files:
            path = os.path.join(REPO_ROOT, rel)
            with open(path, encoding="utf-8") as f:
                text = f.read()
            data = json.loads(text)

            matches = enums_for_framework(data, fw.name)
            if not matches:
                notes.append(f"⚠️  {fw.name}: no version enum found in {rel}")
                continue

            lines = text.splitlines(keepends=True)
            file_touched = False
            for _, enum in matches:
                top = enum[1]
                old_top = old_top or top
                if runner_version in enum:
                    continue  # already present
                if not is_newer(runner_version, top):
                    continue  # not newer than current top
                dropped = enum[-1]
                edit_enum_text(lines, enum, runner_version)
                file_touched = True

            if file_touched:
                with open(path, "w", encoding="utf-8") as f:
                    f.writelines(lines)
                edited_files.append(rel)

        if edited_files:
            changes.append(
                Change(fw.name, old_top, runner_version, dropped, edited_files)
            )
            notes.append(
                f"✅ {fw.name}: {old_top} → {runner_version} "
                f"(dropped {dropped}) in {', '.join(edited_files)}"
            )
        else:
            notes.append(f"➖ {fw.name}: up to date (runner {runner_version})")

    report = "\n".join(notes)
    print(report)

    # PR body
    if changes:
        body_lines = ["Automated framework version sync from the runner repos.", ""]
        for c in changes:
            body_lines.append(
                f"- **{c.framework}**: `{c.old}` → `{c.new}` "
                f"(dropped `{c.dropped}`)"
            )
        pr_body = "\n".join(body_lines)
    else:
        pr_body = ""

    # GitHub Actions outputs
    gh_out = os.environ.get("GITHUB_OUTPUT")
    if gh_out:
        with open(gh_out, "a", encoding="utf-8") as f:
            f.write(f"changed={'true' if changes else 'false'}\n")
            f.write("summary<<EOF\n" + report + "\nEOF\n")
            f.write("pr_body<<EOF\n" + pr_body + "\nEOF\n")

    return 0


if __name__ == "__main__":
    sys.exit(run())
