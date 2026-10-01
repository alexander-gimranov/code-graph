# codeGraphIndexer

CLI для AI-ассистента (Claude, Cursor и аналоги): точечный доступ к большой
кодовой базе **без чтения файлов целиком**.

Три режима:

- **Структурный** — граф зависимостей в SQLite: кто вызывает, наследование,
  методы класса, файлы символа. Обычно ответ быстрее 10 мс.
- **Текстовый** — поиск по файлам: usages, SQL, raw, роуты.
- **Семантический** — поиск по смыслу через эмбеддинги (Voyage AI, OpenAI, Ollama).

Если ты AI-ассистент — читай [codeGraphIndexer.md](codeGraphIndexer.md) или скилл
`code-graph`.

🇬🇧 [Read in English](README.md)

## Настройка

1. Скопируй эту папку в целевой репозиторий как `codeGraphIndexer/` (весь комплект:
   бинарники, `cg.sh`, `install.sh`, skill, документация).
2. Из **корня репозитория** запусти установщик. Он подберёт типичные каталоги с
   кодом, создаст `config.json` и скопирует skill в `.claude/skills` и
   `.cursor/skills`:

   ```bash
   bash codeGraphIndexer/install.sh
   ```

   Опции:

   - `--force` — перезаписать существующий `config.json`
   - `--no-graph` — только конфиг, без `update graph --force`

3. Если пути угаданы неверно, поправь `codeGraphIndexer/config.json` (см.
   [Конфиг](#конфиг)) и выполни:

   ```bash
   bash codeGraphIndexer/cg.sh update graph --force
   ```

В комплекте `.gitignore` — SQLite-кэш и локальный `config.json` внутри
`codeGraphIndexer/`.

## Запуск install.sh

1. Положи комплект в `<корень-репо>/codeGraphIndexer/` (рядом с исходниками
   проекта, не внутри них).
2. Открой терминал в **корне репозитория** — в каталоге, где лежит папка
   `codeGraphIndexer/`.
3. Запусти `install.sh` через **bash** (см. свою ОС ниже). Это не PowerShell и
   не `.cmd`.

**Linux / macOS** — терминал в корне репо:

```bash
cd /path/to/your-repo
bash codeGraphIndexer/install.sh
```

Из папки инструмента:

```bash
cd /path/to/your-repo/codeGraphIndexer
bash install.sh
```

**Windows — Git Bash** (ставится с [Git for Windows](https://git-scm.com/)).
Открой **Git Bash**, перейди в корень репо (пути вида `/c/...`):

```bash
cd /c/Projects/your-repo
bash codeGraphIndexer/install.sh
```

Если `bash` не в `PATH`, укажи Git Bash явно (путь может отличаться):

```bash
cd C:/Projects/your-repo
"/c/Program Files/Git/bin/bash.exe" codeGraphIndexer/install.sh
```

**Windows — WSL** — те же команды, что на Linux; репозиторий на диске `C:` обычно
лежит под `/mnt/c/`:

```bash
cd /mnt/c/Projects/your-repo
bash codeGraphIndexer/install.sh
```

**Python 3** (рекомендуется) обходит репозиторий и добавляет каталог в `scanDirs`
только если там есть файлы нужного языка (например `.rs` в `modules/`, а не PHP
«по названию папки»). На Windows — Python с [python.org](https://www.python.org/),
**Add python.exe to PATH**. Скрипт печатает, что нашёл. Без Python копируется
шаблон `config.example.json` — поправь вручную и запусти `cg.sh update graph --force`.

Примеры (на всех платформах, через bash):

```bash
bash codeGraphIndexer/install.sh --no-graph   # только config и skill
bash codeGraphIndexer/install.sh --force        # перезаписать config.json
bash codeGraphIndexer/install.sh --force --no-graph
```

Если `config.json` уже есть и ты не передал `--force`, скрипт завершится без
изменений.

## Запуск команд

Все команды из **корня репозитория** через `cg.sh` (на Windows выбирается
`codeGraphIndexer.exe`, на Linux/macOS — свой бинарник).

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

Обычные **cmd** и **PowerShell** не запускают `install.sh` / `cg.sh` напрямую —
используй Git Bash или WSL (или явный вызов bash, как в разделе про install).

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

`install.sh` создаёт `config.json` из `config.example.json`. Основные поля:

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

## Примечания

- `db` — только SELECT; multi-statements отключены.
- `schema` — имя таблицы только `[A-Za-z0-9_]+`.
- `--force` — после клона или если база пустая / устарела целиком.
- Если имя известно — сначала `graph`, не чтение файла целиком.

## Лицензия

MIT — см. [LICENSE](LICENSE).
