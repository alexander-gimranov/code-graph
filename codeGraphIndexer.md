# codeGraphIndexer — guide for AI assistants

## What this is

Go binary for navigating a codebase without reading whole files.

- `update graph` — parse PHP, JS/TS, Go, Astro, Rust into SQLite. Without flags,
  refresh only paths from `git status`. With `--force`, re-read every `scanDirs` path.
- Search commands (`graph`, `outline`, `method`, `similar`, …). Before `graph` /
  `similar`, the tool runs `update graph` without `--force`.

Skill: `code-graph` (install into `.claude/skills` and `.cursor/skills`).

## How to run

Always from the **repository root**, through the wrapper (selects OS/arch binary):

```bash
bash codeGraphIndexer/cg.sh <action> <args>
```

Do not hardcode `codeGraphIndexer.exe` or platform-specific names in agent prompts.

## Config

Edit `codeGraphIndexer/config.json` (template: `config.example.json`):

```json
{
  "scanDirs": {
    "rust": ["src"],
    "js": ["frontend/src"]
  },
  "scanExts": {
    "rust": ["rs"],
    "js": ["ts", "tsx", "js", "jsx"]
  },
  "searchDirs": ["src", "frontend/src"],
  "extensions": ["rs", "ts", "tsx"],
  "routeFile": "src/main.rs",
  "mysql": { "host": "localhost", "name": "db", "user": "claude_ro", "pass": "" },
  "embed": { "provider": "", "key": "" }
}
```

## Commands

```bash
bash codeGraphIndexer/cg.sh update graph
bash codeGraphIndexer/cg.sh update graph --force

bash codeGraphIndexer/cg.sh usages MethodName
bash codeGraphIndexer/cg.sh class ClassName
bash codeGraphIndexer/cg.sh raw "text"
bash codeGraphIndexer/cg.sh outline path/to/File.rs
bash codeGraphIndexer/cg.sh method path/to/File.rs foo
bash codeGraphIndexer/cg.sh block path/to/File.ts bar
bash codeGraphIndexer/cg.sh context path/to/File 167 3
bash codeGraphIndexer/cg.sh scss path/to/File.scss .foo
bash codeGraphIndexer/cg.sh entity path/to/Entity.php
bash codeGraphIndexer/cg.sh route methodName
bash codeGraphIndexer/cg.sh sql table_name
bash codeGraphIndexer/cg.sh schema table_name
bash codeGraphIndexer/cg.sh db "SELECT * FROM t LIMIT 5"

bash codeGraphIndexer/cg.sh graph usages MethodName
bash codeGraphIndexer/cg.sh graph methods ClassName
bash codeGraphIndexer/cg.sh graph callers ClassName::method
bash codeGraphIndexer/cg.sh graph deps ClassName
bash codeGraphIndexer/cg.sh graph chain ClassName
bash codeGraphIndexer/cg.sh graph files SymbolName
bash codeGraphIndexer/cg.sh similar "description"
```

## Workflow

**New class / type**

1. `graph files Name`
2. `entity` (PHP) or `outline` + `method`
3. `graph methods Name`

**Where it is called**

1. `graph usages Name`
2. `graph callers Class::method`

**By meaning (name unknown)**

1. `similar "description"` — needs `embed.provider` in `config.json`

**Rules**

- Prefer codeGraphIndexer over reading a whole file.
- Do not invent methods — check with `graph methods` / `entity`.
- Run independent queries in parallel.
- Use `--force` only when the git-based update is not enough (empty or fully stale graph).

## What the graph parses

**PHP:** classes, interfaces, methods, `new`, `::`, `->`, `use`  
**JS/TS:** import, class, functions (including `export function` once), JSX, calls  
**Go:** struct, interface, func, methods, import, calls  
**Rust:** struct/enum/trait, impl, fn, use, instantiation, `Type::f`, macros  
**Astro:** frontmatter + template via the JS parser
