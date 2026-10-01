# codeGraphIndexer

Go CLI для AI-ассистента (Claude, Cursor и аналоги): точечный доступ к большой
кодовой базе **без чтения файлов целиком**.

Три режима:

- **Структурный** — граф зависимостей в SQLite: кто вызывает, наследование,
  методы класса, файлы символа. Обычно ответ быстрее 10 мс.
- **Текстовый** — поиск по файлам: usages, SQL, raw, роуты.
- **Семантический** — поиск по смыслу через эмбеддинги (Voyage AI, OpenAI, Ollama).

Если ты AI-ассистент — читай [codeGraphIndexer.md](codeGraphIndexer.md) или скилл
`code-graph`.

🇬🇧 [Read in English](README.md)

## Установка в проект

В целевом репозитории нужны бинарники и конфиг, не исходники Go.

1. Запусти `bash scripts/build-release.sh` (или возьми готовый `dist/`).
2. Скопируй содержимое `dist/` в целевой репозиторий как `codeGraphIndexer/`.
3. Из корня репо запусти установщик (подробно: `dist/README.md` → **Запуск install.sh**):

   ```bash
   bash codeGraphIndexer/install.sh
   ```

   Создаёт `config.json`, ставит skill и строит граф. Флаги: `--no-graph`, `--force`.
   На Windows те же команды в **Git Bash** или **WSL** (`/c/Projects/...` или
   `/mnt/c/Projects/...`) — см. **Запуск install.sh** в `dist/README.ru.md`.

Все команды запускай из **корня репозитория** через обёртку. Она сама выбирает
бинарник под OS и архитектуру:

```bash
bash codeGraphIndexer/cg.sh <action> <args>
```

На Windows — Git Bash или WSL (не обычные PowerShell/cmd для `install.sh` / `cg.sh`).

## Сборка из исходников

Нужен Go 1.26+.

```bash
go build -o codeGraphIndexer.exe .          # Windows
go build -o codeGraphIndexer .              # Linux / macOS
bash scripts/build-release.sh           # бинарники + skill + README + .gitignore → dist/
```

## Обновление графа

```bash
bash codeGraphIndexer/cg.sh update graph          # только файлы из git status
bash codeGraphIndexer/cg.sh update graph --force  # все файлы из scanDirs
```

- Без `--force` читается `git status --porcelain=v1 -uall -z`. Обновляются только
  пути из `scanDirs` с подходящим расширением. Удалённые и переименованные
  убираются из SQLite.
- Если `git status` падает, команда завершается с ошибкой и не делает полный обход.
- `graph` и `similar` сами вызывают обновление без `--force` перед запросом.

## Конфиг

Правишь `config.json` (шаблон — `config.example.json`):

| Поле | Назначение |
|------|------------|
| `scanDirs` | Язык → каталоги для графа |
| `scanExts` | Язык → расширения (опционально) |
| `searchDirs` / `extensions` | Текстовый поиск |
| `sqlDirs` | Команда `sql` |
| `routeFile` | Команда `route` |
| `mysql` | Read-only MySQL для `schema` / `db` |
| `embed` | Семантический поиск |
| `dbPath` | Путь к SQLite относительно папки `codeGraphIndexer/` |

Пути в `scanDirs` и соседних полях — относительно **родителя** `codeGraphIndexer/`
(корня проекта).

## Команды

```bash
bash codeGraphIndexer/cg.sh update graph
bash codeGraphIndexer/cg.sh update graph --force
bash codeGraphIndexer/cg.sh graph usages MethodName
bash codeGraphIndexer/cg.sh graph methods ClassName
bash codeGraphIndexer/cg.sh graph callers ClassName::method
bash codeGraphIndexer/cg.sh graph deps ClassName
bash codeGraphIndexer/cg.sh graph chain ClassName
bash codeGraphIndexer/cg.sh graph files SymbolName
bash codeGraphIndexer/cg.sh similar "расчёт налога"

bash codeGraphIndexer/cg.sh outline path/to/File.rs
bash codeGraphIndexer/cg.sh method path/to/File.rs foo
bash codeGraphIndexer/cg.sh block path/to/File.ts bar
bash codeGraphIndexer/cg.sh context path/to/File 167 5
bash codeGraphIndexer/cg.sh entity path/to/Entity.php

bash codeGraphIndexer/cg.sh usages MethodName
bash codeGraphIndexer/cg.sh raw "text"
bash codeGraphIndexer/cg.sh sql table_name
bash codeGraphIndexer/cg.sh schema table_name
bash codeGraphIndexer/cg.sh db "SELECT * FROM t LIMIT 5"
```

## Что индексирует граф

| Язык | Символы и ссылки |
|------|------------------|
| PHP | classes, interfaces, methods, `new`, `::`, `->`, `use` |
| JS / TS | import, class, functions (включая `export function`), JSX, calls |
| Go | struct, interface, func, methods, import, calls |
| Rust | struct/enum/trait, impl, fn, use, instantiation, `Type::f`, macros |
| Astro | frontmatter + шаблон через JS-парсер |

## Эмбеддинги

В `config.json`:

```json
"embed": {
  "provider": "voyage",
  "key": "pa-..."
}
```

Провайдеры: `voyage`, `openai`, `ollama`. После настройки один раз запусти
`update graph --force`.

## Структура репозитория

```
codeGraphIndexer/
  main.go
  config.example.json
  cg.sh
  scripts/build-release.sh
  skill/code-graph/
  codeGraphIndexer.md
  internal/...
  dist/                   релизный комплект (в .gitignore)
  scripts/release-README*.md  → dist/README*.md
  scripts/install.sh      → dist/install.sh
  scripts/release.gitignore
```

## Примечания

- `db` — только SELECT; multi-statements отключены.
- `schema` — имя таблицы только `[A-Za-z0-9_]+`.
- `--force` — после клона или если база пустая / устарела целиком.
- Если имя известно — сначала `graph`, не чтение файла целиком.

## Лицензия

MIT — см. [LICENSE](LICENSE).
