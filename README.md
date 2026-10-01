# codeGraphIndexer

Go CLI that gives an AI assistant (Claude, Cursor, and similar tools) precise,
low-token access to a large codebase — without reading entire files.

Three search modes:

- **Structural** — SQLite dependency graph: callers, inheritance, class methods,
  files for a symbol. Typical answer time is under 10 ms after the graph is warm.
- **Textual** — on-the-fly search across project files: usages, SQL, raw text, routes.
- **Semantic** — vector similarity (Voyage AI, OpenAI, or Ollama) when you know
  the meaning but not the name.

If you are an AI assistant, read [codeGraphIndexer.md](codeGraphIndexer.md) or use the
`code-graph` skill.

🇷🇺 [Читать на русском](README.ru.md)

## Install into a project

You only need the binaries and config in the consumer repo — not this Go source.

1. Run `bash scripts/build-release.sh` (or use a published `dist/` bundle).
2. Copy everything from `dist/` into the target repository as `codeGraphIndexer/`.
3. From the repo root, run the installer (see `dist/README.md` → **Running install.sh**):

   ```bash
   bash codeGraphIndexer/install.sh
   ```

   It writes `config.json`, installs the skill, and builds the graph. Use
   `--no-graph` or `--force` when needed. On Windows run the same commands in
   **Git Bash** or **WSL** (`/c/Projects/...` or `/mnt/c/Projects/...`) — see
   **Running install.sh** in `dist/README.md`.

Always run commands from the **repository root** through the wrapper. It picks
the binary for the current OS and architecture:

```bash
bash codeGraphIndexer/cg.sh <action> <args>
```

On Windows use Git Bash or WSL (not plain PowerShell/cmd for `install.sh` / `cg.sh`).

## Build from source

Requires Go 1.26+.

```bash
go build -o codeGraphIndexer.exe .          # Windows
go build -o codeGraphIndexer .              # Linux / macOS (generic name)
bash scripts/build-release.sh           # binaries + skill + README + .gitignore → dist/
```

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

Edit `config.json` (start from `config.example.json`):

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

## SQLite schema

```sql
files       (id, path, mtime, lang)
symbols     (id, file_id, type, name, full_name, line, visibility, is_static)
refs        (id, file_id, from_full_name, to_name, ref_type, line)
embeddings  (symbol_id, vector, norm)   -- when embed.provider is set
```

## Semantic search

Set in `config.json`:

```json
"embed": {
  "provider": "voyage",
  "key": "pa-..."
}
```

Supported providers: `voyage`, `openai`, `ollama`. Then run
`update graph --force` once so new symbols get vectors. Voyage and OpenAI send
texts in batches; Ollama embeds one prompt at a time.

## Layout

```
codeGraphIndexer/
  main.go                 CLI entry
  config.example.json     Template for a new project
  cg.sh                   OS/arch binary picker
  scripts/build-release.sh
  skill/code-graph/       Agent skill source
  codeGraphIndexer.md         Short guide for assistants
  internal/
    config/ ignore/ parse/ graph/ search/ embed/ mysql/
  dist/                   Release bundle (gitignored)
  scripts/release-README*.md  Copied to dist/README*.md (no build instructions)
  scripts/install.sh      Copied to dist/install.sh
  scripts/release.gitignore  Template for dist/.gitignore
```

## Notes

- `db` accepts SELECT only. Multi-statements are disabled at the driver level.
- `schema` accepts table names that match `[A-Za-z0-9_]+`.
- Use `--force` after a clone or when the SQLite file is missing or fully stale.
- Prefer `graph usages` over full-file reads when the symbol name is known.

## License

MIT — see [LICENSE](LICENSE).
