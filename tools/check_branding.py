#!/usr/bin/env python3
"""Reject retired branding in tracked paths, bytes and nested release packages.

Signed shortcut semantics are verified separately by the macOS signing checks.
Git history, repository metadata and externally installed copies are not scanned.
"""
from __future__ import annotations
import argparse
import io
import pathlib
import re
import subprocess
import tarfile
import zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
# Keep the retired spelling out of the current source tree itself.
RETIRED = bytes.fromhex('7461696c636c6970').decode('ascii')
PATTERNS = [re.compile(re.escape(RETIRED.encode(enc)), re.I)
            for enc in ('utf-8', 'utf-16-le', 'utf-16-be', 'utf-32-le', 'utf-32-be')]
MAX_MEMBER = 512 * 1024 * 1024

def check_bytes(name: str, data: bytes, failures: list[str], counts: dict[str, int], depth: int = 0) -> None:
    counts['entries'] += 1
    if any(p.search(name.encode('utf-8')) for p in PATTERNS):
        failures.append('retired filename: ' + name)
    if any(p.search(data) for p in PATTERNS):
        failures.append('retired content: ' + name)
    if depth > 5:
        raise ValueError('archive nesting limit: ' + name)
    if name.lower().endswith('.zip'):
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            for entry in archive.infolist():
                if entry.file_size > MAX_MEMBER: raise ValueError('archive member too large')
                if not entry.is_dir():
                    check_bytes(name + '!' + entry.filename, archive.read(entry), failures, counts, depth + 1)
    elif name.lower().endswith(('.tar.gz', '.tgz', '.tar')):
        with tarfile.open(fileobj=io.BytesIO(data), mode='r:*') as archive:
            for entry in archive:
                if entry.size > MAX_MEMBER: raise ValueError('archive member too large')
                if entry.isfile():
                    stream = archive.extractfile(entry)
                    if stream is None: raise ValueError('unreadable archive member')
                    check_bytes(name + '!' + entry.name, stream.read(), failures, counts, depth + 1)

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('extra', nargs='*', type=pathlib.Path, help='additional package files or directories')
    args = parser.parse_args()
    raw = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT)
    paths = {ROOT / name for name in raw.decode('utf-8').split('\0') if name}
    for extra in args.extra:
        paths.update(p for p in extra.rglob('*') if p.is_file()) if extra.is_dir() else paths.add(extra)
    failures: list[str] = []
    counts = {'entries': 0, 'files': 0}
    for path in sorted(paths):
        if not path.is_file():
            failures.append('tracked file missing: ' + str(path))
            continue
        try:
            name = path.relative_to(ROOT).as_posix()
        except ValueError:
            name = path.name
        check_bytes(name, path.read_bytes(), failures, counts)
        counts['files'] += 1
    for failure in failures: print(failure)
    print(f"Branding audit: {counts['files']} files, {counts['entries']} entries including archive members, {len(failures)} failures")
    return 1 if failures else 0

if __name__ == '__main__':
    raise SystemExit(main())
