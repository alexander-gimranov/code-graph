package graph

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"codegraphindexer/internal/config"
	"codegraphindexer/internal/embed"
	"codegraphindexer/internal/ignore"
	"codegraphindexer/internal/parse"

	_ "modernc.org/sqlite"
)

type DB struct {
	sql *sql.DB
	cfg *config.Config
}

func Open(cfg *config.Config) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	d := &DB{sql: db, cfg: cfg}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error { return d.sql.Close() }

func (d *DB) migrate() error {
	_, err := d.sql.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id    INTEGER PRIMARY KEY,
			path  TEXT UNIQUE,
			mtime INTEGER,
			lang  TEXT
		);
		CREATE TABLE IF NOT EXISTS symbols (
			id         INTEGER PRIMARY KEY,
			file_id    INTEGER,
			type       TEXT,
			name       TEXT,
			full_name  TEXT,
			line       INTEGER,
			visibility TEXT,
			is_static  INTEGER DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS refs (
			id             INTEGER PRIMARY KEY,
			file_id        INTEGER,
			from_full_name TEXT,
			to_name        TEXT,
			ref_type       TEXT,
			line           INTEGER
		);
		CREATE TABLE IF NOT EXISTS embeddings (
			symbol_id INTEGER PRIMARY KEY,
			vector    TEXT,
			norm      REAL
		);
		CREATE INDEX IF NOT EXISTS idx_sym_name ON symbols(name);
		CREATE INDEX IF NOT EXISTS idx_sym_full ON symbols(full_name);
		CREATE INDEX IF NOT EXISTS idx_sym_file ON symbols(file_id);
		CREATE INDEX IF NOT EXISTS idx_ref_to   ON refs(to_name);
		CREATE INDEX IF NOT EXISTS idx_ref_from ON refs(from_full_name);
		CREATE INDEX IF NOT EXISTS idx_ref_file ON refs(file_id);
	`)
	if err != nil {
		return err
	}
	// Add norm column for older DBs.
	var dummy string
	err = d.sql.QueryRow(`SELECT norm FROM embeddings LIMIT 1`).Scan(&dummy)
	if err != nil && strings.Contains(err.Error(), "no such column") {
		_, err = d.sql.Exec(`ALTER TABLE embeddings ADD COLUMN norm REAL`)
		return err
	}
	return nil
}

type workItem struct {
	rel  string
	abs  string
	lang string
	op   string // "parse" or "delete"
}

// Update updates the graph. force=true re-reads all scan files; force=false uses git status.
// quiet suppresses per-file and summary output (used when graph/similar auto-update).
func (d *DB) Update(force, quiet bool) error {
	patterns := ignore.Load(d.cfg.RootDir)
	var items []workItem
	seenOnDisk := map[string]bool{}

	if force {
		var err error
		items, seenOnDisk, err = d.collectAll(patterns)
		if err != nil {
			return err
		}
	} else {
		var err error
		items, err = d.collectGit(patterns)
		if err != nil {
			return err
		}
	}

	updated := 0
	deleted := 0
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 8 {
		workers = 8
	}

	type parsed struct {
		item workItem
		res  parse.Result
		err  error
	}

	jobs := make(chan workItem, len(items))
	results := make(chan parsed, len(items))

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range jobs {
				if it.op == "delete" {
					results <- parsed{item: it}
					continue
				}
				fn := parse.Registry[it.lang]
				if fn == nil {
					results <- parsed{item: it, err: fmt.Errorf("no parser for %s", it.lang)}
					continue
				}
				data, err := os.ReadFile(it.abs)
				if err != nil {
					results <- parsed{item: it, err: err}
					continue
				}
				results <- parsed{item: it, res: fn(string(data), it.rel)}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	for _, it := range items {
		jobs <- it
	}
	close(jobs)

	for p := range results {
		if p.err != nil {
			fmt.Fprintf(os.Stderr, "  error %s: %v\n", p.item.rel, p.err)
			continue
		}
		if p.item.op == "delete" {
			if err := d.deletePath(p.item.rel); err != nil {
				fmt.Fprintf(os.Stderr, "  delete %s: %v\n", p.item.rel, err)
				continue
			}
			deleted++
			if !quiet {
				fmt.Printf("  deleted: %s\n", p.item.rel)
			}
			continue
		}
		if err := d.upsertFile(p.item, p.res); err != nil {
			fmt.Fprintf(os.Stderr, "  upsert %s: %v\n", p.item.rel, err)
			continue
		}
		updated++
		if !quiet {
			fmt.Printf("  parsed: %s\n", p.item.rel)
		}
	}

	if force {
		rows, err := d.sql.Query(`SELECT id, path FROM files`)
		if err != nil {
			return err
		}
		var stale []struct {
			id   int64
			path string
		}
		for rows.Next() {
			var id int64
			var path string
			if err := rows.Scan(&id, &path); err != nil {
				rows.Close()
				return err
			}
			if !seenOnDisk[path] {
				stale = append(stale, struct {
					id   int64
					path string
				}{id, path})
			}
		}
		rows.Close()
		for _, s := range stale {
			if err := d.deleteFileID(s.id); err != nil {
				return err
			}
			deleted++
			if !quiet {
				fmt.Printf("  deleted: %s\n", s.path)
			}
		}
	}

	if err := d.indexEmbeddings(quiet); err != nil {
		fmt.Fprintf(os.Stderr, "embeddings: %v\n", err)
	}

	if !quiet {
		fmt.Printf("\nDone. updated: %d, deleted: %d\n", updated, deleted)
		fmt.Printf("Graph: %s\n", d.cfg.DBPath)
	}
	return nil
}

func (d *DB) collectAll(patterns []string) ([]workItem, map[string]bool, error) {
	seen := map[string]bool{}
	var items []workItem
	for lang, dirs := range d.cfg.ScanDirs {
		exts := map[string]bool{}
		for _, e := range d.cfg.ScanExts[lang] {
			exts[e] = true
		}
		for _, dir := range dirs {
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				continue
			}
			err := ignore.WalkFiltered(dir, d.cfg.RootDir, patterns, func(abs, rel string) error {
				ext := strings.TrimPrefix(filepath.Ext(abs), ".")
				if !exts[ext] {
					return nil
				}
				seen[rel] = true
				items = append(items, workItem{rel: rel, abs: abs, lang: lang, op: "parse"})
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return items, seen, nil
}

func (d *DB) collectGit(patterns []string) ([]workItem, error) {
	root := strings.TrimRight(d.cfg.RootDir, `\/`)
	cmd := exec.Command("git", "-C", root, "status", "--porcelain=v1", "-uall", "-z")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git status failed: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	langOf := d.langLookup()
	var items []workItem
	seen := map[string]bool{}

	parts := strings.Split(string(out), "\x00")
	i := 0
	for i < len(parts) {
		entry := parts[i]
		i++
		if entry == "" {
			continue
		}
		if len(entry) < 3 {
			continue
		}
		xy := entry[:2]
		path := entry[3:]

		// Rename/copy: next NUL field is the other path.
		var oldPath string
		if xy[0] == 'R' || xy[0] == 'C' || (len(xy) > 1 && (xy[1] == 'R' || xy[1] == 'C')) {
			if i < len(parts) {
				// porcelain -z for rename: "XY oldpath\x00newpath\x00" OR "XY newpath\x00oldpath"?
				// Actually: status --porcelain -z for rename is: "R  old\x00new\x00"
				// Wait: man git-status: "For paths with rename or copy status, the first path is the old path,
				// the second path is the new path."
				// And format is: XY PATH\0 or for rename XY ORIG_PATH\0PATH\0
				// So entry[3:] is ORIG path, next is new path.
				oldPath = path
				if i < len(parts) {
					path = parts[i]
					i++
				}
			}
		}

		addParse := func(p string) {
			p = filepath.ToSlash(p)
			if ignore.IsIgnored(p, patterns) || seen[p+"|parse"] {
				return
			}
			lang, ok := langOf(p)
			if !ok {
				return
			}
			seen[p+"|parse"] = true
			items = append(items, workItem{
				rel: p, abs: config.AbsPath(d.cfg.RootDir, p), lang: lang, op: "parse",
			})
		}
		addDelete := func(p string) {
			p = filepath.ToSlash(p)
			if seen[p+"|delete"] {
				return
			}
			seen[p+"|delete"] = true
			items = append(items, workItem{rel: p, op: "delete"})
		}

		isDeleted := strings.Contains(xy, "D")
		isUntracked := xy == "??" || xy == "!"
		isRenamed := strings.ContainsAny(xy, "RC")

		if isRenamed && oldPath != "" {
			addDelete(oldPath)
			addParse(path)
			continue
		}
		if isDeleted {
			addDelete(path)
			continue
		}
		if isUntracked || strings.ContainsAny(xy, "MADRCU") {
			addParse(path)
		}
	}
	return items, nil
}

func (d *DB) langLookup() func(rel string) (string, bool) {
	type rule struct {
		prefix string
		lang   string
		exts   map[string]bool
	}
	var rules []rule
	root := strings.TrimRight(d.cfg.RootDir, `\/`)
	for lang, dirs := range d.cfg.ScanDirs {
		exts := map[string]bool{}
		for _, e := range d.cfg.ScanExts[lang] {
			exts[e] = true
		}
		for _, dir := range dirs {
			rel, err := filepath.Rel(root, dir)
			if err != nil {
				rel = dir
			}
			rel = filepath.ToSlash(rel)
			rules = append(rules, rule{prefix: rel, lang: lang, exts: exts})
		}
	}
	return func(rel string) (string, bool) {
		ext := strings.TrimPrefix(filepath.Ext(rel), ".")
		for _, r := range rules {
			if (rel == r.prefix || strings.HasPrefix(rel, r.prefix+"/")) && r.exts[ext] {
				return r.lang, true
			}
		}
		return "", false
	}
}

func (d *DB) deletePath(rel string) error {
	var id sql.NullInt64
	err := d.sql.QueryRow(`SELECT id FROM files WHERE path = ?`, rel).Scan(&id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return d.deleteFileID(id.Int64)
}

func (d *DB) deleteFileID(id int64) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM embeddings WHERE symbol_id IN (SELECT id FROM symbols WHERE file_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM symbols WHERE file_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM refs WHERE file_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) upsertFile(it workItem, res parse.Result) error {
	st, err := os.Stat(it.abs)
	mtime := int64(0)
	if err == nil {
		mtime = st.ModTime().Unix()
	}

	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var fileID int64
	err = tx.QueryRow(`SELECT id FROM files WHERE path = ?`, it.rel).Scan(&fileID)
	if err == sql.ErrNoRows {
		r, err := tx.Exec(`INSERT INTO files (path, mtime, lang) VALUES (?, ?, ?)`, it.rel, mtime, it.lang)
		if err != nil {
			return err
		}
		fileID, _ = r.LastInsertId()
	} else if err != nil {
		return err
	} else {
		// Preserve embeddings by type+full_name before deleting symbols.
		type embKey struct{ typ, full string }
		saved := map[embKey]struct {
			vector string
			norm   sql.NullFloat64
		}{}
		rows, err := tx.Query(`
			SELECT s.type, s.full_name, e.vector, e.norm
			FROM symbols s JOIN embeddings e ON e.symbol_id = s.id
			WHERE s.file_id = ?`, fileID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var typ, full, vec string
			var norm sql.NullFloat64
			if err := rows.Scan(&typ, &full, &vec, &norm); err != nil {
				rows.Close()
				return err
			}
			saved[embKey{typ, full}] = struct {
				vector string
				norm   sql.NullFloat64
			}{vec, norm}
		}
		rows.Close()

		if _, err := tx.Exec(`DELETE FROM embeddings WHERE symbol_id IN (SELECT id FROM symbols WHERE file_id = ?)`, fileID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM refs WHERE file_id = ?`, fileID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE files SET mtime = ?, lang = ? WHERE id = ?`, mtime, it.lang, fileID); err != nil {
			return err
		}

		insSym, err := tx.Prepare(`INSERT INTO symbols (file_id, type, name, full_name, line, visibility, is_static) VALUES (?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer insSym.Close()
		insEmb, err := tx.Prepare(`INSERT INTO embeddings (symbol_id, vector, norm) VALUES (?, ?, ?)`)
		if err != nil {
			return err
		}
		defer insEmb.Close()
		insRef, err := tx.Prepare(`INSERT INTO refs (file_id, from_full_name, to_name, ref_type, line) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer insRef.Close()

		for _, s := range res.Symbols {
			resIns, err := insSym.Exec(fileID, s.Type, s.Name, s.FullName, s.Line, s.Visibility, s.IsStatic)
			if err != nil {
				return err
			}
			sid, _ := resIns.LastInsertId()
			if e, ok := saved[embKey{s.Type, s.FullName}]; ok {
				var norm any
				if e.norm.Valid {
					norm = e.norm.Float64
				}
				if _, err := insEmb.Exec(sid, e.vector, norm); err != nil {
					return err
				}
			}
		}
		for _, ref := range res.Refs {
			if _, err := insRef.Exec(fileID, ref.FromFullName, ref.ToName, ref.RefType, ref.Line); err != nil {
				return err
			}
		}
		return tx.Commit()
	}

	// New file path
	insSym, err := tx.Prepare(`INSERT INTO symbols (file_id, type, name, full_name, line, visibility, is_static) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insSym.Close()
	insRef, err := tx.Prepare(`INSERT INTO refs (file_id, from_full_name, to_name, ref_type, line) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insRef.Close()
	for _, s := range res.Symbols {
		if _, err := insSym.Exec(fileID, s.Type, s.Name, s.FullName, s.Line, s.Visibility, s.IsStatic); err != nil {
			return err
		}
	}
	for _, ref := range res.Refs {
		if _, err := insRef.Exec(fileID, ref.FromFullName, ref.ToName, ref.RefType, ref.Line); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) indexEmbeddings(quiet bool) error {
	client := embed.New(d.cfg.Embed)
	if !client.Enabled() {
		return nil
	}
	if _, err := d.sql.Exec(`DELETE FROM embeddings WHERE symbol_id NOT IN (SELECT id FROM symbols)`); err != nil {
		return err
	}
	rows, err := d.sql.Query(`
		SELECT s.id, s.type, s.full_name, f.path
		FROM symbols s
		LEFT JOIN embeddings e ON e.symbol_id = s.id
		JOIN files f ON f.id = s.file_id
		WHERE e.symbol_id IS NULL`)
	if err != nil {
		return err
	}
	type sym struct {
		id   int64
		text string
	}
	var list []sym
	for rows.Next() {
		var id int64
		var typ, full, path string
		if err := rows.Scan(&id, &typ, &full, &path); err != nil {
			rows.Close()
			return err
		}
		list = append(list, sym{id, typ + ": " + full + " (" + path + ")"})
	}
	rows.Close()
	if len(list) == 0 {
		return nil
	}
	if !quiet {
		fmt.Printf("\nEmbeddings: indexing %d new symbols...\n", len(list))
	}
	texts := make([]string, len(list))
	for i, s := range list {
		texts[i] = s.text
	}
	vecs, err := client.EmbedBatch(texts)
	if err != nil {
		return err
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO embeddings (symbol_id, vector, norm) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	indexed := 0
	for i, s := range list {
		if i >= len(vecs) || vecs[i] == nil {
			continue
		}
		b, _ := json.Marshal(vecs[i])
		norm := embed.L2Norm(vecs[i])
		if _, err := stmt.Exec(s.id, string(b), norm); err != nil {
			return err
		}
		indexed++
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if !quiet {
		fmt.Printf("Embeddings: indexed %d symbols\n", indexed)
	}
	return nil
}
