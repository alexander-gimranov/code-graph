#!/usr/bin/env bash
# Generate config.json for this repo, install agent skill, optional first graph build.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
FORCE=0
RUN_GRAPH=1

usage() {
  echo "Usage: bash install.sh [--force] [--no-graph]"
  echo "  --force     overwrite existing config.json"
  echo "  --no-graph  skip initial: cg.sh update graph --force"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force) FORCE=1; shift ;;
    --no-graph) RUN_GRAPH=0; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ ! -f "$DIR/config.example.json" ]]; then
  echo "config.example.json not found in $DIR" >&2
  exit 1
fi

if [[ -f "$DIR/config.json" && "$FORCE" -ne 1 ]]; then
  echo "config.json already exists. Use --force to overwrite, or edit it manually." >&2
  exit 1
fi

write_config() {
  "$@" - "$DIR" "$ROOT" <<'PY'
import json
import os
import sys
from pathlib import Path

tool_dir = Path(sys.argv[1])
root = Path(sys.argv[2]).resolve()

cfg = json.loads((tool_dir / "config.example.json").read_text(encoding="utf-8"))

SKIP = {
    ".git", ".idea", ".vscode", "node_modules", "vendor", "target", "dist", "build",
    "codeGraphIndexer", ".cursor", ".claude", "__pycache__", ".next", "coverage",
}
LANG_EXTS = {
    "php": {"php"},
    "js": {"js", "jsx", "ts", "tsx", "mjs", "cjs"},
    "go": {"go"},
    "rust": {"rs"},
}
DEFAULT_SCAN_EXTS = {
    "php": ["php"],
    "js": ["ts", "tsx", "js", "jsx"],
    "go": ["go"],
    "rust": ["rs"],
}
CANDIDATES = {
    "php": ["src", "app", "lib", "modules", "public", "database"],
    "js": [
        "resources/js", "src", "frontend", "client", "web",
        "apps/web/src", "apps/web/app", "apps/web/backoffice",
    ],
    "go": ["cmd", "internal", "pkg"],
    "rust": [
        "platform-api/src", "modules", "middleware", "common", "src", "crates",
    ],
}


def file_ext(name: str) -> str:
    i = name.rfind(".")
    return name[i + 1 :].lower() if i >= 0 else ""


def count_ext_files(rel_dir: str, exts: set[str], max_depth: int = 16) -> int:
    base = root / rel_dir
    if not base.is_dir():
        return 0
    total = 0
    for dirpath, dirnames, filenames in os.walk(base):
        p = Path(dirpath)
        try:
            rel = p.relative_to(root)
        except ValueError:
            break
        if any(part in SKIP for part in rel.parts):
            dirnames.clear()
            continue
        depth = len(p.relative_to(base).parts)
        if depth > max_depth:
            dirnames.clear()
            continue
        dirnames[:] = [d for d in dirnames if d not in SKIP]
        for fn in filenames:
            if file_ext(fn) in exts:
                total += 1
    return total


def dedupe_shallow(roots: list[str]) -> list[str]:
    keep: list[str] = []
    for r in sorted(set(roots), key=lambda x: (x.count("/"), len(x))):
        if any(r != k and r.startswith(k + "/") for k in keep):
            continue
        keep = [k for k in keep if not k.startswith(r + "/")]
        keep.append(r)
    return sorted(keep, key=str.lower)


def scan_roots_for_lang(lang: str) -> list[str]:
    exts = LANG_EXTS[lang]
    roots: list[str] = []
    for cand in CANDIDATES[lang]:
        if count_ext_files(cand, exts) > 0:
            roots.append(cand)

    if lang == "js":
        for app in sorted(root.glob("apps/*")):
            if not app.is_dir() or app.name in SKIP:
                continue
            for sub in ("src", "app", "lib", "backoffice"):
                rel = (app / sub).relative_to(root).as_posix()
                if (app / sub).is_dir() and count_ext_files(rel, exts) > 0:
                    roots.append(rel)

    if lang in ("rust", "go", "php"):
        for sub in sorted(root.iterdir()):
            if not sub.is_dir() or sub.name in SKIP:
                continue
            rel1 = sub.relative_to(root).as_posix()
            if count_ext_files(rel1, exts) > 0 and rel1 not in roots:
                roots.append(rel1)
            for leaf in ("src", "lib", "app"):
                p = sub / leaf
                if not p.is_dir():
                    continue
                rel2 = p.relative_to(root).as_posix()
                if count_ext_files(rel2, exts) > 0 and rel2 not in roots:
                    roots.append(rel2)

    if lang == "go" and (root / "go.mod").is_file() and not roots:
        if count_ext_files(".", exts, max_depth=8) > 0:
            roots.append(".")

    return dedupe_shallow(roots)


def pick_file(*candidates: str) -> str:
    for c in candidates:
        if (root / c).is_file():
            return c
    return cfg.get("routeFile", "")


def pick_dirs(*candidates: str) -> list[str]:
    return [c for c in candidates if (root / c).is_dir()]


scan: dict[str, list[str]] = {}
notes: list[str] = []

for lang in ("php", "js", "go", "rust"):
    dirs = scan_roots_for_lang(lang)
    if dirs:
        scan[lang] = dirs
        sample_ext = sorted(LANG_EXTS[lang])[0]
        for d in dirs:
            n = count_ext_files(d, LANG_EXTS[lang])
            notes.append(f"  {lang}: {d} ({n} .{sample_ext} files under tree)")

if scan:
    cfg["scanDirs"] = scan
else:
    notes.append("  (no scanDirs detected — edit config.json manually)")

cfg["scanExts"] = {
    lang: DEFAULT_SCAN_EXTS.get(lang, cfg.get("scanExts", {}).get(lang, [lang]))
    for lang in cfg["scanDirs"]
}

search: list[str] = []
for dirs in cfg["scanDirs"].values():
    search.extend(dirs)
search.extend(pick_dirs("tests", "test", "resources", "specs", "docs", "scripts", "infra"))
cfg["searchDirs"] = sorted(set(search), key=str.lower) or cfg.get("searchDirs", [])

sql_candidates = [
    "database/migrations", "migrations", "db/migrations", "sql", "seed",
    "middleware/store/migrations", "middleware/store",
]
sql = [
    c for c in sql_candidates
    if count_ext_files(c, {"sql"}) > 0 or (root / c).is_dir()
]
if not sql:
    sql = list(cfg["searchDirs"][:8])
cfg["sqlDirs"] = sorted(set(sql))

route = pick_file("routes/web.php", "routes/api.php")
if route:
    cfg["routeFile"] = route

out = tool_dir / "config.json"
out.write_text(json.dumps(cfg, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
print(f"Wrote {out}")
print("scanDirs:", json.dumps(cfg["scanDirs"], ensure_ascii=False))
if notes:
    print("Detection (paths must contain matching source files):")
    print("\n".join(notes))
PY
}

py_ok() {
  command -v "$1" >/dev/null 2>&1 && "$1" -c "import sys" >/dev/null 2>&1
}

PY_CMD=()
if py_ok python3; then
  PY_CMD=(python3)
elif py_ok python; then
  PY_CMD=(python)
elif command -v py >/dev/null 2>&1 && py -3 -c "import sys" >/dev/null 2>&1; then
  PY_CMD=(py -3)
fi

if ((${#PY_CMD[@]})); then
  write_config "${PY_CMD[@]}"
else
  cp "$DIR/config.example.json" "$DIR/config.json"
  echo "Python not found; copied config.example.json → config.json." >&2
  echo "Edit config.json manually (scanDirs, searchDirs, …)." >&2
fi

for dest in .claude/skills/code-graph .cursor/skills/code-graph; do
  mkdir -p "$ROOT/$dest"
  cp "$DIR/skill/code-graph/SKILL.md" "$ROOT/$dest/SKILL.md"
  echo "Installed skill → $ROOT/$dest/SKILL.md"
done

if [[ "$RUN_GRAPH" -eq 1 ]]; then
  echo "Building graph (first run may take a while)..."
  (cd "$ROOT" && bash "$DIR/cg.sh" update graph --force)
else
  echo "Skipped graph build. Run from repo root:"
  echo "  bash codeGraphIndexer/cg.sh update graph --force"
fi
