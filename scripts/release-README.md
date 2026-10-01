# codeGraphIndexer

CLI for AI assistants (Claude, Cursor, and similar tools): precise, low-token
access to a large codebase without reading entire files.

Three search modes:

- **Structural** — SQLite dependency graph: callers, inheritance, class methods,
  files for a symbol. Typical answer time is under 10 ms after the graph is warm.
- **Textual** — on-the-fly search across project files: usages, SQL, raw text, routes.
- **Semantic** — vector similarity (Voyage AI, OpenAI, or Ollama) when you know
  the meaning but not the name.

If you are an AI assistant, read [codeGraphIndexer.md](codeGraphIndexer.md) or use the
`code-graph` skill.

🇷🇺 [Читать на русском](README.ru.md)

## Setup

1. Copy this folder into the target repository as `codeGraphIndexer/` (everything
   in this bundle: binaries, `cg.sh`, `install.sh`, skill, docs).
2. From the **repository root**, run the installer. It detects common source
   directories, writes `config.json`, and copies the skill into
   `.claude/skills` and `.cursor/skills`:

   ```bash
   bash codeGraphIndexer/install.sh
   ```

   Options:

   - `--force` — overwrite an existing `config.json`
   - `--no-graph` — only configure; skip `update graph --force`

3. If detection missed paths, edit `codeGraphIndexer/config.json` (see
   [Configuration](#configuration)) and run:

   ```bash
   bash codeGraphIndexer/cg.sh update graph --force
   ```

The bundled `.gitignore` ignores SQLite cache and local `config.json` inside
`codeGraphIndexer/`.

## Running install.sh

1. Put this bundle at `<repo-root>/codeGraphIndexer/` (sibling to your app
   sources, not inside them).
2. Open a terminal at the **repository root** — the directory that contains
   `codeGraphIndexer/`.
3. Run `install.sh` with **bash** (see your OS below). The script is not a
   PowerShell or `.cmd` file.

**Linux / macOS** — Terminal at the repo root:

```bash
cd /path/to/your-repo
bash codeGraphIndexer/install.sh
```

From inside the tool folder:

```bash
cd /path/to/your-repo/codeGraphIndexer
bash install.sh
```

**Windows — Git Bash** (installed with [Git for Windows](https://git-scm.com/)).
Open **Git Bash**, go to the repo root (use `/c/...` paths):

```bash
cd /c/Projects/your-repo
bash codeGraphIndexer/install.sh
```

If `bash` is not on `PATH`, call Git Bash explicitly (adjust the path if Git
is installed elsewhere):

```bash
cd C:/Projects/your-repo
"/c/Program Files/Git/bin/bash.exe" codeGraphIndexer/install.sh
```

**Windows — WSL** — use the Linux commands from your distro, with the repo on
the Windows drive mounted under `/mnt/`:

```bash
cd /mnt/c/Projects/your-repo
bash codeGraphIndexer/install.sh
```

**Python 3** (recommended) scans the repo and fills `scanDirs` only where matching
source files exist (for example `.rs` under `modules/`, not PHP just because the
folder name exists). On Windows, install Python from [python.org](https://www.python.org/)
and enable **Add python.exe to PATH**. The script prints what it detected. If
Python is missing, it copies `config.example.json` to `config.json` — edit by hand,
then run `cg.sh update graph --force`.

Common variants (same on all platforms, via bash):

```bash
bash codeGraphIndexer/install.sh --no-graph   # config + skill only
bash codeGraphIndexer/install.sh --force        # overwrite existing config.json
bash codeGraphIndexer/install.sh --force --no-graph
```

If `config.json` already exists and you run without `--force`, the script exits
and leaves the current file unchanged.

## Running commands

Always from the **repository root** through `cg.sh` (picks `codeGraphIndexer.exe`
on Windows or the matching Linux/macOS binary elsewhere).

**Linux / macOS:**

```bash
cd /path/to/your-repo
bash codeGraphIndexer/cg.sh <action> <args>
```

**Windows — Git Bash:**

```bash
cd /c/Projects/your-repo
bash codeGraphIndexer/cg.sh <action> <args>
```

**Windows — WSL:**

```bash
cd /mnt/c/Projects/your-repo
bash codeGraphIndexer/cg.sh <action> <args>
```

Plain **cmd** or **PowerShell** do not run `install.sh` / `cg.sh` directly; use
Git Bash or WSL, or invoke Git’s bash as in the install section above.

## How the graph updates

```bash
bash codeGraphIndexer/cg.sh update graph          # only files from git status
bash codeGraphIndexer/cg.sh update graph --force  # re-read every scan path
```

- Without `--force`, the tool runs
  `git status --porcelain=v1 -uall -z` and refreshes only those paths that fall
  under `scanDirs` with a matching extension. Deleted and renamed files are
  removed from the SQLite graph.
- If `git status` fails, the command exits with an error. It does not fall back
  to a full scan.
- `graph` and `similar` call the non-force update automatically before the query.

## Configuration

`install.sh` creates `config.json` from `config.example.json`. Main fields:

| Field | Purpose |
|-------|---------|
| `scanDirs` | Language → directories indexed into the graph |
| `scanExts` | Language → file extensions (optional; default is the language key) |
| `searchDirs` / `extensions` | Text search (`usages`, `raw`, …) |
| `sqlDirs` | Directories for the `sql` command |
| `routeFile` | File for the `route` command |
| `mysql` | Read-only MySQL for `schema` / `db` |
| `embed` | Optional semantic search (`voyage`, `openai`, `ollama`) |
| `dbPath` | SQLite path relative to the `codeGraphIndexer/` folder |

Paths in `scanDirs`, `searchDirs`, and similar fields are relative to the
**parent** of `codeGraphIndexer/` (the project root).

## Commands

```bash
# Graph
bash codeGraphIndexer/cg.sh update graph
bash codeGraphIndexer/cg.sh update graph --force
bash codeGraphIndexer/cg.sh graph usages MethodName
bash codeGraphIndexer/cg.sh graph methods ClassName
bash codeGraphIndexer/cg.sh graph callers ClassName::method
bash codeGraphIndexer/cg.sh graph deps ClassName
bash codeGraphIndexer/cg.sh graph chain ClassName
bash codeGraphIndexer/cg.sh graph files SymbolName
bash codeGraphIndexer/cg.sh similar "calculate tax"

# File fragments
bash codeGraphIndexer/cg.sh outline path/to/File.rs
bash codeGraphIndexer/cg.sh method path/to/File.rs foo
bash codeGraphIndexer/cg.sh block path/to/File.ts bar
bash codeGraphIndexer/cg.sh context path/to/File 167 5
bash codeGraphIndexer/cg.sh entity path/to/Entity.php
bash codeGraphIndexer/cg.sh scss path/to/File.scss .selector

# Text and DB
bash codeGraphIndexer/cg.sh usages MethodName
bash codeGraphIndexer/cg.sh class ClassName
bash codeGraphIndexer/cg.sh raw "text"
bash codeGraphIndexer/cg.sh sql table_name
bash codeGraphIndexer/cg.sh schema table_name
bash codeGraphIndexer/cg.sh db "SELECT * FROM t LIMIT 5"
bash codeGraphIndexer/cg.sh route methodName
```

## What the parsers index

| Language | Symbols and refs |
|----------|------------------|
| PHP | classes, interfaces, methods, `new`, `::`, `->`, `use` |
| JS / TS | imports, classes, functions (including `export function`), JSX, calls |
| Go | structs, interfaces, funcs, methods, imports, calls |
| Rust | struct/enum/trait, impl, fn, use, instantiation, `Type::f`, macros |
| Astro | frontmatter + template via the JS parser |

## Semantic search

Set in `config.json`:

```json
"embed": {
  "provider": "voyage",
  "key": "pa-..."
}
```

Supported providers: `voyage`, `openai`, `ollama`. Then run
`update graph --force` once so new symbols get vectors.

## Notes

- `db` accepts SELECT only. Multi-statements are disabled at the driver level.
- `schema` accepts table names that match `[A-Za-z0-9_]+`.
- Use `--force` after a clone or when the SQLite file is missing or fully stale.
- Prefer `graph usages` over full-file reads when the symbol name is known.

## License

MIT — see [LICENSE](LICENSE).
