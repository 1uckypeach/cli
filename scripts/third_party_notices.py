#!/usr/bin/env python3
# Copyright (c) 2026 Lark Technologies Pte. Ltd.
# SPDX-License-Identifier: MIT

"""Generate a deterministic, fail-closed third-party notices document."""

from __future__ import annotations

import argparse
import dataclasses
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Iterable


MAX_FILE_BYTES = 2 * 1024 * 1024
MAX_TOTAL_BYTES = 16 * 1024 * 1024
LICENSE_BASENAMES = ("LICENSE", "COPYING")
NOTICE_BASENAMES = ("NOTICE",)
PROHIBITED_LICENSE_WORDS = ("GPL", "LGPL", "AGPL", "SSPL", "GENERAL PUBLIC LICENSE", "SERVER SIDE PUBLIC")
MIT_REQUIRED_CLAUSES = (
    "permission is hereby granted, free of charge",
    "the above copyright notice and this permission notice shall be included in all copies or substantial portions of the software",
    "the software is provided as is",
    "in no event shall the authors or copyright holders be liable for any claim",
)
APACHE_REQUIRED_CLAUSES = (
    "apache license",
    "version 2.0",
    "terms and conditions for use, reproduction, and distribution",
    "grant of copyright license",
    "grant of patent license",
    "redistribution",
    "submission of contributions",
    "trademarks",
    "disclaimer of warranty",
    "limitation of liability",
    "accepting warranty or additional liability",
)
ISC_REQUIRED_CLAUSES = (
    "permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted",
    "the above copyright notice and this permission notice shall be included in all copies",
    "the author disclaims all warranties with regard to this software",
    "in no event shall the author be liable for any special, direct, indirect, or consequential damages",
)
BSD_REQUIRED_CLAUSES = (
    "redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met",
    "redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer",
    "redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer",
    "this software is provided by the copyright holders and contributors as is",
)
BSD_LIABILITY_CLAUSES = (
    "in no event shall the copyright holder or contributors be liable for any direct, indirect, incidental, special, exemplary, or consequential damages",
    "in no event shall the copyright owner or contributors be liable for any direct, indirect, incidental, special, exemplary, or consequential damages",
)
RELEASE_TARGETS = (
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("linux", "riscv64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
)
NODE_OS_ALIASES = {"windows": "win32"}
NODE_CPU_ALIASES = {"amd64": "x64"}
NODE_RELEASE_TARGETS = tuple(
    (NODE_OS_ALIASES.get(goos, goos), NODE_CPU_ALIASES.get(goarch, goarch)) for goos, goarch in RELEASE_TARGETS
)


class NoticeError(RuntimeError):
    """A dependency cannot be safely included in a notices document."""


@dataclasses.dataclass(frozen=True)
class Component:
    name: str
    version: str
    source: str
    license_id: str
    copyright: str
    license_text: str
    notice_text: str = ""


@dataclasses.dataclass
class ReadBudget:
    total: int = 0

    def charge(self, size: int) -> None:
        if size > MAX_FILE_BYTES:
            raise NoticeError(f"dependency file exceeds {MAX_FILE_BYTES} byte limit")
        self.total += size
        if self.total > MAX_TOTAL_BYTES:
            raise NoticeError(f"dependency files exceed {MAX_TOTAL_BYTES} byte total limit")


def _within(root: Path, target: Path) -> bool:
    try:
        return os.path.commonpath((str(root), str(target))) == str(root)
    except ValueError:
        return False


def _validate_dependency_path(root: Path, path: Path) -> Path:
    """Reject symlinks and paths that resolve outside a dependency root."""
    root = root.absolute()
    path = path.absolute()
    if not _within(root, path):
        raise NoticeError(f"dependency path escapes its root: {path}")
    try:
        relative = path.relative_to(root)
    except ValueError as error:
        raise NoticeError(f"dependency path escapes its root: {path}") from error

    current = root
    for part in (".", *relative.parts):
        if part != ".":
            current = current / part
        try:
            mode = os.lstat(current).st_mode
        except OSError as error:
            raise NoticeError(f"cannot lstat dependency path: {current}") from error
        if stat.S_ISLNK(mode):
            raise NoticeError(f"symlinked dependency path is not allowed: {current}")

    resolved_root = Path(os.path.realpath(root))
    resolved_path = Path(os.path.realpath(path))
    if not _within(resolved_root, resolved_path):
        raise NoticeError(f"dependency path resolves outside its root: {path}")
    return resolved_path


def safe_read_text(root: Path, path: Path, budget: ReadBudget) -> str:
    """Read one UTF-8 dependency file after containment and size validation."""
    resolved = _validate_dependency_path(root, path)
    try:
        file_stat = os.lstat(resolved)
    except OSError as error:
        raise NoticeError(f"cannot lstat dependency file: {path}") from error
    if not stat.S_ISREG(file_stat.st_mode):
        raise NoticeError(f"dependency file is not a regular file: {path}")
    budget.charge(file_stat.st_size)
    try:
        with open(resolved, "rb") as handle:
            data = handle.read(MAX_FILE_BYTES + 1)
    except OSError as error:
        raise NoticeError(f"cannot read dependency file: {path}") from error
    if len(data) != file_stat.st_size or len(data) > MAX_FILE_BYTES:
        raise NoticeError(f"dependency file changed while reading: {path}")
    try:
        return data.decode("utf-8")
    except UnicodeDecodeError as error:
        raise NoticeError(f"dependency file is not valid UTF-8: {path}") from error


def _read_json(root: Path, path: Path, budget: ReadBudget) -> dict:
    try:
        value = json.loads(safe_read_text(root, path, budget))
    except json.JSONDecodeError as error:
        raise NoticeError(f"invalid JSON in dependency metadata: {path}") from error
    if not isinstance(value, dict):
        raise NoticeError(f"dependency metadata is not an object: {path}")
    return value


def _allowed_document_name(name: str, basenames: Iterable[str]) -> bool:
    upper_name = name.upper()
    for basename in basenames:
        if not upper_name.startswith(basename):
            continue
        if upper_name == basename:
            return True
        suffix = name[len(basename):]
        if suffix.lower() in (".md", ".txt"):
            return True
        if suffix.startswith("-") and re.fullmatch(r"-[A-Za-z0-9._-]+", suffix):
            return True
    return False


def _document_text(root: Path, basenames: Iterable[str], budget: ReadBudget, required: bool) -> str:
    documents = []
    _validate_dependency_path(root, root)
    try:
        candidates = sorted(
            (path for path in root.iterdir() if _allowed_document_name(path.name, basenames)),
            key=lambda path: path.name,
        )
    except OSError as error:
        raise NoticeError(f"cannot list dependency documents: {root}") from error
    for candidate in candidates:
        # lstat is deliberately performed even when this is the final path.
        try:
            os.lstat(candidate)
        except FileNotFoundError:
            continue
        except OSError as error:
            raise NoticeError(f"cannot inspect dependency document: {candidate}") from error
        documents.append(safe_read_text(root, candidate, budget))
    if required and not documents:
        raise NoticeError(f"missing required license document in {root}")
    return "\n\n".join(documents)


def _reject_prohibited(value: str) -> None:
    normalized = value.upper()
    if any(word in normalized for word in PROHIBITED_LICENSE_WORDS):
        raise NoticeError(f"prohibited license: {value}")


def _normalized_license_text(value: str) -> str:
    return re.sub(r"[\"“”]", "", re.sub(r"\s+", " ", value)).casefold()


def _has_required_clauses(license_text: str, clauses: Iterable[str]) -> bool:
    normalized = _normalized_license_text(license_text)
    return all(clause in normalized for clause in clauses)


def _detect_bsd_license(license_text: str) -> str:
    normalized = _normalized_license_text(license_text)
    if "all advertising materials" in normalized:
        raise NoticeError("unsupported BSD-4-Clause license")
    if (
        not _has_required_clauses(license_text, BSD_REQUIRED_CLAUSES)
        or not any(clause in normalized for clause in BSD_LIABILITY_CLAUSES)
    ):
        raise NoticeError("incomplete BSD-2-Clause or BSD-3-Clause license")
    if "neither the name of" in normalized:
        return "BSD-3-Clause"
    return "BSD-2-Clause"


def _detect_license_ids(license_text: str) -> set[str]:
    _reject_prohibited(license_text)
    normalized = _normalized_license_text(license_text)
    detected = set()
    if _has_required_clauses(license_text, APACHE_REQUIRED_CLAUSES):
        detected.add("Apache-2.0")
    if _has_required_clauses(license_text, MIT_REQUIRED_CLAUSES):
        detected.add("MIT")
    if _has_required_clauses(license_text, ISC_REQUIRED_CLAUSES):
        detected.add("ISC")
    if "redistribution and use in source and binary forms" in normalized:
        detected.add(_detect_bsd_license(license_text))
    return detected


def normalize_license_id(value: object, license_text: str) -> str:
    """Return an allowed SPDX-like identifier, or fail closed."""
    declared = ""
    if isinstance(value, str):
        declared = value.strip()
    elif isinstance(value, dict) and isinstance(value.get("type"), str):
        declared = value["type"].strip()
    detected = _detect_license_ids(license_text)
    if declared:
        _reject_prohibited(declared)
        normalized = declared.upper().replace(" ", "")
        if normalized in {"MIT", "MITLICENSE"}:
            expected = "MIT"
        elif normalized in {"ISC", "ISCLICENSE"}:
            expected = "ISC"
        elif normalized.startswith("APACHE-2") or normalized in {"APACHE2.0", "APACHELICENSE2.0"}:
            expected = "Apache-2.0"
        elif normalized.startswith("BSD-2"):
            expected = "BSD-2-Clause"
        elif normalized.startswith("BSD-3"):
            expected = "BSD-3-Clause"
        elif normalized in {"BSD", "BSDLICENSE"}:
            return _detect_bsd_license(license_text)
        else:
            raise NoticeError(f"unknown or unsupported license: {declared}")
        if expected not in detected:
            raise NoticeError(f"license text does not match declared license: {declared}")
        return expected
    if not detected:
        raise NoticeError("license cannot be identified from dependency metadata or text")
    return " OR ".join(sorted(detected))


def _copyright_lines(*texts: str) -> str:
    lines = []
    for text in texts:
        for line in text.splitlines():
            stripped = line.strip()
            normalized = stripped.lower()
            if normalized.startswith(("copyright notice", "copyright license")):
                continue
            if normalized.startswith("copyright") or stripped.startswith("©"):
                lines.append(stripped)
    return "\n".join(dict.fromkeys(lines)) or "Not specified"


def _repository_source(metadata: dict, fallback: str) -> str:
    repository = metadata.get("repository")
    if isinstance(repository, dict):
        repository = repository.get("url")
    if not isinstance(repository, str) or not repository.strip():
        return fallback
    source = repository.strip()
    if source.startswith("git+"):
        source = source[4:]
    if source.endswith(".git"):
        source = source[:-4]
    return source


def _component_from_package(
    package_dir: Path, budget: ReadBudget, *, name: str, version: str, source: str, declared_license: object
) -> Component:
    license_text = _document_text(package_dir, LICENSE_BASENAMES, budget, required=True)
    notice_text = _document_text(package_dir, NOTICE_BASENAMES, budget, required=False)
    return Component(
        name=name,
        version=version,
        source=source,
        license_id=normalize_license_id(declared_license, license_text),
        copyright=_copyright_lines(license_text, notice_text),
        license_text=license_text,
        notice_text=notice_text,
    )


def component_from_node_package(package_dir: Path, budget: ReadBudget) -> Component:
    metadata = _read_json(package_dir, package_dir / "package.json", budget)
    name, version = metadata.get("name"), metadata.get("version")
    if not isinstance(name, str) or not name or not isinstance(version, str) or not version:
        raise NoticeError(f"missing name or version in dependency metadata: {package_dir}")
    return _component_from_package(
        package_dir,
        budget,
        name=name,
        version=version,
        source=_repository_source(metadata, f"https://www.npmjs.com/package/{name}/v/{version}"),
        declared_license=metadata.get("license"),
    )


def component_from_go_module(module: dict, budget: ReadBudget) -> Component:
    name, version, directory = module.get("Path"), module.get("Version"), module.get("Dir")
    if not isinstance(name, str) or not isinstance(version, str) or not isinstance(directory, str):
        raise NoticeError("go list returned a module with missing path, version, or directory")
    return _component_from_package(
        Path(directory), budget, name=name, version=version, source=f"https://pkg.go.dev/{name}@{version}", declared_license=None
    )


def _parse_json_stream(value: str) -> list[dict]:
    decoder = json.JSONDecoder()
    position = 0
    records = []
    while position < len(value):
        while position < len(value) and value[position].isspace():
            position += 1
        if position == len(value):
            break
        try:
            record, position = decoder.raw_decode(value, position)
        except json.JSONDecodeError as error:
            raise NoticeError("invalid JSON from Go command") from error
        if not isinstance(record, dict):
            raise NoticeError("invalid module record from Go command")
        records.append(record)
    return records


def _go_runtime_module_records(repo_root: Path) -> list[dict]:
    modules: dict[tuple[str, str], dict] = {}
    with tempfile.TemporaryDirectory(prefix="third-party-notices-go-") as temporary:
        temp_root = Path(temporary)
        _copy_input_file(repo_root, temp_root, "go.mod")
        _copy_input_file(repo_root, temp_root, "go.sum")
        modfile = temp_root / "go.mod"
        for goos, goarch in RELEASE_TARGETS:
            environment = dict(os.environ, CGO_ENABLED="0", GOOS=goos, GOARCH=goarch)
            try:
                result = subprocess.run(
                    ["go", "list", "-mod=mod", f"-modfile={modfile}", "-deps", "-json", "."],
                    cwd=repo_root,
                    capture_output=True,
                    text=True,
                    check=True,
                    env=environment,
                )
            except (OSError, subprocess.CalledProcessError) as error:
                raise NoticeError(f"go list failed for {goos}/{goarch}") from error
            for package in _parse_json_stream(result.stdout):
                module = package.get("Module")
                if not isinstance(module, dict) or module.get("Main"):
                    continue
                name, version = module.get("Path"), module.get("Version")
                if not isinstance(name, str) or not isinstance(version, str):
                    raise NoticeError(f"go list returned invalid module metadata for {goos}/{goarch}")
                modules[(name, version)] = module
    if not modules:
        raise NoticeError("go list did not find any third-party runtime modules")
    return [modules[key] for key in sorted(modules)]


def collect_go_components(repo_root: Path, budget: ReadBudget) -> list[Component]:
    records = _go_runtime_module_records(repo_root)
    missing_directories = [record for record in records if not isinstance(record.get("Dir"), str)]
    if missing_directories:
        modules = ", ".join(f"{record.get('Path')}@{record.get('Version')}" for record in missing_directories)
        raise NoticeError(f"go list did not locate module source: {modules}")
    return [component_from_go_module(record, budget) for record in records]


def _copy_input_file(repo_root: Path, destination: Path, name: str) -> None:
    source = repo_root / name
    _validate_dependency_path(repo_root, source)
    try:
        shutil.copy2(source, destination / name)
    except OSError as error:
        raise NoticeError(f"cannot copy {name} into isolated npm directory") from error


def _node_package_directories(node_modules: Path) -> Iterable[Path]:
    _validate_dependency_path(node_modules, node_modules)
    for current, directories, filenames in os.walk(node_modules, topdown=True, followlinks=False):
        current_path = Path(current)
        kept = []
        for directory in directories:
            path = current_path / directory
            try:
                is_link = stat.S_ISLNK(os.lstat(path).st_mode)
            except OSError as error:
                raise NoticeError(f"cannot inspect npm dependency directory: {path}") from error
            if is_link:
                raise NoticeError(f"symlinked npm dependency directory is not allowed: {path}")
            kept.append(directory)
        directories[:] = kept
        if "package.json" not in filenames:
            continue
        parent = current_path.parent
        is_unscoped = parent.name == "node_modules"
        is_scoped = parent.parent.name == "node_modules" and parent.name.startswith("@")
        if is_unscoped or is_scoped:
            yield current_path


def collect_node_components(repo_root: Path, budget: ReadBudget) -> list[Component]:
    with tempfile.TemporaryDirectory(prefix="third-party-notices-") as temporary:
        temp_root = Path(temporary)
        components: dict[tuple[str, str, str], Component] = {}
        for node_os, node_cpu in NODE_RELEASE_TARGETS:
            target_root = temp_root / f"{node_os}-{node_cpu}"
            target_root.mkdir()
            _copy_input_file(repo_root, target_root, "package.json")
            _copy_input_file(repo_root, target_root, "package-lock.json")
            try:
                subprocess.run(
                    [
                        "npm",
                        "ci",
                        "--ignore-scripts",
                        "--omit=dev",
                        f"--os={node_os}",
                        f"--cpu={node_cpu}",
                    ],
                    cwd=target_root,
                    capture_output=True,
                    text=True,
                    check=True,
                )
            except (OSError, subprocess.CalledProcessError) as error:
                raise NoticeError(f"npm ci failed for {node_os}/{node_cpu}") from error
            for directory in _node_package_directories(target_root / "node_modules"):
                component = component_from_node_package(directory, budget)
                key = (component.name, component.version, component.source)
                existing = components.get(key)
                if existing and existing != component:
                    raise NoticeError(
                        f"npm dependency metadata differs across release targets: {component.name}@{component.version}"
                    )
                components[key] = component
        return [components[key] for key in sorted(components)]


def render_notices(components: Iterable[Component]) -> str:
    lines = ["# Third-Party Notices", ""]
    for component in sorted(components, key=lambda item: (item.name.lower(), item.name, item.version, item.source)):
        lines.extend((
            f"## {component.name} {component.version}",
            "",
            f"- Component: {component.name}",
            f"- Version: {component.version}",
            f"- Source: {component.source}",
            f"- License: {component.license_id}",
            f"- Copyright: {component.copyright}",
            "",
            "### License Text",
            "```text",
            component.license_text.rstrip("\n"),
            "```",
        ))
        if component.notice_text:
            lines.extend(("", "### NOTICE", "```text", component.notice_text.rstrip("\n"), "```"))
        lines.append("")
    return "\n".join(lines)


def generate(repo_root: Path, output: Path) -> str:
    budget = ReadBudget()
    components = collect_go_components(repo_root, budget) + collect_node_components(repo_root, budget)
    document = render_notices(components)
    try:
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(document, encoding="utf-8", newline="\n")
    except OSError as error:
        raise NoticeError(f"cannot write output: {output}") from error
    return document


def check(repo_root: Path, output: Path) -> None:
    if not output.is_file():
        raise NoticeError(f"notices output does not exist: {output}")
    with tempfile.TemporaryDirectory(prefix="third-party-notices-check-") as temporary:
        generated = Path(temporary) / "THIRD_PARTY_NOTICES.md"
        generate(repo_root, generated)
        try:
            expected = output.read_bytes()
            actual = generated.read_bytes()
        except OSError as error:
            raise NoticeError("cannot read notices output for comparison") from error
    if actual != expected:
        raise NoticeError("third-party notices are out of date; run generate with --output")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("generate", "check"))
    parser.add_argument("--output", required=True, type=Path, help="explicit path to THIRD_PARTY_NOTICES.md")
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parent.parent)
    args = parser.parse_args(argv)
    try:
        if args.command == "generate":
            generate(args.repo_root.resolve(), args.output.resolve())
        else:
            check(args.repo_root.resolve(), args.output.resolve())
    except NoticeError as error:
        print(f"third_party_notices: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
