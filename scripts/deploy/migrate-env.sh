#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SOURCE_FILE="${1:-$REPO_ROOT/.env.deploy}"
TARGET_FILE="${2:-$REPO_ROOT/.env.deploy.migrated}"
TEMPLATE_FILE="$REPO_ROOT/.env.deploy.example"
PYTHON_BIN="${PYTHON_BIN:-python3}"

[[ -f "$SOURCE_FILE" ]] || { printf 'source env is missing: %s\n' "$SOURCE_FILE" >&2; exit 1; }
[[ -f "$TEMPLATE_FILE" ]] || { printf 'template env is missing: %s\n' "$TEMPLATE_FILE" >&2; exit 1; }
if [[ -e "$TARGET_FILE" && "${FORCE:-0}" != "1" ]]; then
  printf 'target exists; set FORCE=1 to replace it: %s\n' "$TARGET_FILE" >&2
  exit 1
fi

"$PYTHON_BIN" - "$SOURCE_FILE" "$TEMPLATE_FILE" "$TARGET_FILE" <<'PY'
import os
import re
import stat
import sys
import tempfile

source_path, template_path, target_path = sys.argv[1:]
entry = re.compile(r"^[ \t]*(?:export[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*=(.*)$")

def read_values(path):
    values = {}
    duplicates = set()
    with open(path, "r", encoding="utf-8-sig") as handle:
        for raw in handle:
            match = entry.match(raw.rstrip("\r\n"))
            if not match:
                continue
            key, value = match.groups()
            if key in values:
                duplicates.add(key)
            values[key] = value
    if duplicates:
        raise SystemExit("duplicate keys in source: " + ", ".join(sorted(duplicates)))
    return values

source = read_values(source_path)
output = []
template_keys = set()
with open(template_path, "r", encoding="utf-8") as handle:
    for raw in handle:
        match = entry.match(raw.rstrip("\r\n"))
        if not match:
            output.append(raw)
            continue
        key, default = match.groups()
        template_keys.add(key)
        output.append(f"{key}={source.get(key, default)}\n")

unknown = sorted(set(source) - template_keys)
if unknown:
    output.extend(["\n", "# Preserved legacy keys; review and remove after migration.\n"])
    output.extend(f"{key}={source[key]}\n" for key in unknown)

directory = os.path.dirname(os.path.abspath(target_path))
os.makedirs(directory, exist_ok=True)
fd, temp_path = tempfile.mkstemp(prefix=".env-migrate.", dir=directory, text=True)
try:
    with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as handle:
        handle.writelines(output)
    os.chmod(temp_path, stat.S_IRUSR | stat.S_IWUSR)
    os.replace(temp_path, target_path)
finally:
    if os.path.exists(temp_path):
        os.unlink(temp_path)

print(f"wrote {target_path}; inherited={len(set(source) & template_keys)} legacy={len(unknown)}")
PY
