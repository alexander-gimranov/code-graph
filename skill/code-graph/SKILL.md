---
name: code-graph
description: >-
  Navigates a codebase via codeGraphIndexer (SQLite dependency graph, targeted
  file reads, optional semantic search). Use when finding symbols, call sites,
  methods, inheritance, SQL/table usage, or reading a method body without
  loading whole files.
---

# code-graph

Always run from the repository root via the shell wrapper (picks the binary for
this OS/arch):

```bash
bash codeGraphIndexer/cg.sh <action> <args>
```

Do not call `codeGraphIndexer.exe` or platform-specific binaries directly.

Prefer these commands over reading entire files.

## Update graph

`graph` and `similar` call `update graph` automatically (git-changed files only).

```bash
bash codeGraphIndexer/cg.sh update graph          # files from git status
bash codeGraphIndexer/cg.sh update graph --force  # re-read all scan dirs
```

Use `--force` after a fresh clone, or when the SQLite graph is empty or fully stale.

## Known name

| Need | Command |
|------|---------|
| Where used | `graph usages Name` |
| Who calls | `graph callers Class::method` |
| Class methods | `graph methods ClassName` |
| Where defined | `graph files Symbol` |
| Extends chain | `graph chain ClassName` |
| Outgoing refs | `graph deps ClassName` |

## Unknown name

```bash
bash codeGraphIndexer/cg.sh similar "what the code does"
```

Requires `embed.provider` in `codeGraphIndexer/config.json`.

## Read a fragment

1. `outline path/to/File` — method list with lines
2. `method path/to/File.go Foo` or `block path/to/File.ts bar` — body
3. `context path/to/File 167 3` — ±N lines
4. `entity path/to/Entity.php` — PHP fields and constructor
5. `sql table_name` / `schema table_name` — SQL and DESCRIBE
6. `raw "text"` — plain text search

## Rules

- Do not invent methods or fields; verify with `entity` or `graph methods`.
- Run independent queries in parallel.
- Do not use `Read` for a whole file when a fragment command is enough.
- Always use `bash codeGraphIndexer/cg.sh`, never a hardcoded `.exe` path.
