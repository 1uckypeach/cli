#!/usr/bin/env python3
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
RELEASE_TARGETS = (
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("linux", "riscv64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
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


def _detect_bsd_license(license_text: str) -> str:
    upper_text = license_text.upper()
    if "ALL ADVERTISING MATERIALS" in upper_text:
        raise NoticeError("unsupported BSD-4-Clause license")
    if "REDISTRIBUTION AND USE IN SOURCE AND BINARY FORMS" not in upper_text:
        raise NoticeError("license cannot be identified as BSD-2-Clause or BSD-3-Clause")
    if "NEITHER THE NAME OF" in upper_text:
        return "BSD-3-Clause"
    return "BSD-2-Clause"


def normalize_license_id(value: object, license_text: str) -> str:
    """Return an allowed SPDX-like identifier, or fail closed."""
    declared = ""
    if isinstance(value, str):
        declared = value.strip()
    elif isinstance(value, dict) and isinstance(value.get("type"), str):
        declared = value["type"].strip()
    if declared:
        _reject_prohibited(declared)
        normalized = declared.upper().replace(" ", "")
        if normalized in {"MIT", "MITLICENSE"}:
            return "MIT"
        if normalized in {"ISC", "ISCLICENSE"}:
            return "ISC"
        if normalized.startswith("APACHE-2") or normalized in {"APACHE2.0", "APACHELICENSE2.0"}:
            return "Apache-2.0"
        if normalized.startswith("BSD-2"):
            return "BSD-2-Clause"
        if normalized.startswith("BSD-3"):
            return "BSD-3-Clause"
        if normalized in {"BSD", "BSDLICENSE"}:
            return _detect_bsd_license(license_text)
        raise NoticeError(f"unknown or unsupported license: {declared}")

    _reject_prohibited(license_text)
    upper_text = license_text.upper()
    detected = []
    if "APACHE LICENSE" in upper_text and "VERSION 2.0" in upper_text:
        detected.append("Apache-2.0")
    if "PERMISSION IS HEREBY GRANTED, FREE OF CHARGE" in upper_text:
        detected.append("MIT")
    if "PERMISSION TO USE, COPY, MODIFY, AND/OR DISTRIBUTE" in upper_text:
        detected.append("ISC")
    if "REDISTRIBUTION AND USE IN SOURCE AND BINARY FORMS" in upper_text:
        detected.append(_detect_bsd_license(license_text))
    if not detected:
        raise NoticeError("license cannot be identified from dependency metadata or text")
    return " OR ".join(sorted(detected))


def _copyright_lines(*texts: str) -> str:
    lines = []
    for text in texts:
        lines.extend(
            line.strip()
            for line in text.splitlines()
            if line.lstrip().lower().startswith("copyright") or line.lstrip().startswith("©")
        )
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


def component_from_node_package(package_dir: Path, budget: ReadBudget) -> Component:
    metadata = _read_json(package_dir, package_dir / "package.json", budget)
    name, version = metadata.get("name"), metadata.get("version")
    if not isinstance(name, str) or not name or not isinstance(version, str) or not version:
        raise NoticeError(f"missing name or version in dependency metadata: {package_dir}")
    license_text = _document_text(package_dir, LICENSE_BASENAMES, budget, required=True)
    notice_text = _document_text(package_dir, NOTICE_BASENAMES, budget, required=False)
    license_id = normalize_license_id(metadata.get("license"), license_text)
    return Component(
        name=name,
        version=version,
        source=_repository_source(metadata, f"https://www.npmjs.com/package/{name}/v/{version}"),
        license_id=license_id,
        copyright=_copyright_lines(license_text, notice_text),
        license_text=license_text,
        notice_text=notice_text,
    )


def component_from_go_module(module: dict, budget: ReadBudget) -> Component:
    name, version, directory = module.get("Path"), module.get("Version"), module.get("Dir")
    if not isinstance(name, str) or not isinstance(version, str) or not isinstance(directory, str):
        raise NoticeError("go list returned a module with missing path, version, or directory")
    module_dir = Path(directory)
    license_text = _document_text(module_dir, LICENSE_BASENAMES, budget, required=True)
    notice_text = _document_text(module_dir, NOTICE_BASENAMES, budget, required=False)
    license_id = normalize_license_id(None, license_text)
    return Component(
        name=name,
        version=version,
        source=f"https://pkg.go.dev/{name}@{version}",
        license_id=license_id,
        copyright=_copyright_lines(license_text, notice_text),
        license_text=license_text,
        notice_text=notice_text,
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
            raise NoticeError("invalid JSON from go list -m -json all") from error
        if not isinstance(record, dict):
            raise NoticeError("invalid module record from go list -m -json all")
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
        try:
            downloaded = subprocess.run(
                ["go", "mod", "download", "-json", "all"], cwd=repo_root, capture_output=True, text=True, check=True
            )
        except (OSError, subprocess.CalledProcessError) as error:
            raise NoticeError("go mod download -json all failed while locating module source") from error
        locations = {
            (record.get("Path"), record.get("Version")): record.get("Dir")
            for record in _parse_json_stream(downloaded.stdout)
            if isinstance(record.get("Dir"), str)
        }
        for record in missing_directories:
            directory = locations.get((record.get("Path"), record.get("Version")))
            if not isinstance(directory, str):
                raise NoticeError(f"cannot locate module source for {record.get('Path')}@{record.get('Version')}")
            record["Dir"] = directory
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
        _copy_input_file(repo_root, temp_root, "package.json")
        _copy_input_file(repo_root, temp_root, "package-lock.json")
        try:
            subprocess.run(
                ["npm", "ci", "--ignore-scripts", "--omit=dev"], cwd=temp_root, capture_output=True, text=True, check=True
            )
        except (OSError, subprocess.CalledProcessError) as error:
            raise NoticeError("npm ci --ignore-scripts --omit=dev failed") from error
        return [component_from_node_package(directory, budget) for directory in _node_package_directories(temp_root / "node_modules")]


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
