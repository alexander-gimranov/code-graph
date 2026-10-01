package graph

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"codegraphindexer/internal/embed"
)

func (d *DB) GraphUsages(name string) error {
	rows, err := d.sql.Query(`
		SELECT r.ref_type, r.from_full_name, f.path, r.line
		FROM refs r JOIN files f ON f.id = r.file_id
		WHERE r.to_name = ? OR r.to_name LIKE ?
		ORDER BY f.path, r.line`, name, "%::"+name)
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var refType, from, path string
		var line int
		if err := rows.Scan(&refType, &from, &path, &line); err != nil {
			return err
		}
		fmt.Printf("%-14s %-50s %s:%d\n", refType, from, path, line)
		n++
	}
	if n == 0 {
		fmt.Printf("No usages: %s\n", name)
		return nil
	}
	fmt.Printf("\n(%d usages)\n", n)
	return nil
}

func (d *DB) GraphMethods(name string) error {
	rows, err := d.sql.Query(`
		SELECT s.type, s.full_name, s.visibility, s.is_static, f.path, s.line
		FROM symbols s JOIN files f ON f.id = s.file_id
		WHERE s.full_name LIKE ? OR s.name = ?
		ORDER BY s.line`, name+"::%", name)
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var typ, full, vis, path string
		var isStatic, line int
		if err := rows.Scan(&typ, &full, &vis, &isStatic, &path, &line); err != nil {
			return err
		}
		static := ""
		if isStatic != 0 {
			static = "static "
		}
		fmt.Printf("%-10s %-10s %s%-50s %s:%d\n", typ, vis, static, full, path, line)
		n++
	}
	if n == 0 {
		fmt.Printf("No symbols: %s\n", name)
		return nil
	}
	fmt.Printf("\n(%d symbols)\n", n)
	return nil
}

func (d *DB) GraphCallers(name string) error {
	rows, err := d.sql.Query(`
		SELECT r.ref_type, r.from_full_name, f.path, r.line
		FROM refs r JOIN files f ON f.id = r.file_id
		WHERE r.to_name = ? AND r.ref_type IN ('call','static_call','jsx')
		ORDER BY f.path, r.line`, name)
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var refType, from, path string
		var line int
		if err := rows.Scan(&refType, &from, &path, &line); err != nil {
			return err
		}
		fmt.Printf("%-12s %-50s %s:%d\n", refType, from, path, line)
		n++
	}
	if n == 0 {
		fmt.Printf("No callers: %s\n", name)
		return nil
	}
	fmt.Printf("\n(%d callers)\n", n)
	return nil
}

func (d *DB) GraphDeps(name string) error {
	rows, err := d.sql.Query(`
		SELECT r.ref_type, r.to_name, r.from_full_name, f.path, r.line
		FROM refs r JOIN files f ON f.id = r.file_id
		WHERE r.from_full_name LIKE ? OR r.from_full_name = ?
		ORDER BY r.ref_type, r.to_name`, name+"::%", name)
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var refType, to, from, path string
		var line int
		if err := rows.Scan(&refType, &to, &from, &path, &line); err != nil {
			return err
		}
		fmt.Printf("%-14s %-40s from %-50s %s:%d\n", refType, to, from, path, line)
		n++
	}
	if n == 0 {
		fmt.Printf("No deps: %s\n", name)
		return nil
	}
	fmt.Printf("\n(%d deps)\n", n)
	return nil
}

func (d *DB) GraphChain(name string) error {
	visited := map[string]bool{}
	queue := []string{name}
	type row struct {
		refType, to, from, path string
		line                    int
	}
	var rowsOut []row
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		rows, err := d.sql.Query(`
			SELECT r.ref_type, r.to_name, f.path, r.line
			FROM refs r JOIN files f ON f.id = r.file_id
			WHERE r.from_full_name = ? AND r.ref_type IN ('extends','implements')`, cur)
		if err != nil {
			return err
		}
		for rows.Next() {
			var r row
			r.from = cur
			if err := rows.Scan(&r.refType, &r.to, &r.path, &r.line); err != nil {
				rows.Close()
				return err
			}
			rowsOut = append(rowsOut, r)
			queue = append(queue, r.to)
		}
		rows.Close()
	}
	if len(rowsOut) == 0 {
		fmt.Printf("No chain: %s\n", name)
		return nil
	}
	for _, r := range rowsOut {
		fmt.Printf("%-14s %-30s → %-30s %s:%d\n", r.refType, r.from, r.to, r.path, r.line)
	}
	return nil
}

func (d *DB) GraphFiles(name string) error {
	rows, err := d.sql.Query(`
		SELECT DISTINCT f.path, 'symbol' as kind, s.type, s.line
		FROM symbols s JOIN files f ON f.id = s.file_id
		WHERE s.name = ? OR s.full_name = ?`, name, name)
	if err != nil {
		return err
	}
	type entry struct {
		path, kind, typ string
		line            int
	}
	var all []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.path, &e.kind, &e.typ, &e.line); err != nil {
			rows.Close()
			return err
		}
		all = append(all, e)
	}
	rows.Close()

	rows, err = d.sql.Query(`
		SELECT DISTINCT f.path, 'ref' as kind, r.ref_type, r.line
		FROM refs r JOIN files f ON f.id = r.file_id
		WHERE r.to_name = ?`, name)
	if err != nil {
		return err
	}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.path, &e.kind, &e.typ, &e.line); err != nil {
			rows.Close()
			return err
		}
		all = append(all, e)
	}
	rows.Close()

	if len(all) == 0 {
		fmt.Printf("Not found in graph: %s\n", name)
		return nil
	}
	for _, e := range all {
		fmt.Printf("%-8s %-14s %s:%d\n", e.kind, e.typ, e.path, e.line)
	}
	fmt.Printf("\n(%d entries)\n", len(all))
	return nil
}

type similarHit struct {
	sim             float64
	typ, full, path string
	line            int
}

func (d *DB) Similar(term string) error {
	client := embed.New(d.cfg.Embed)
	if !client.Enabled() {
		fmt.Println("Embeddings not configured.")
		fmt.Println("Set embed.provider and embed.key in config.json")
		os.Exit(1)
	}
	if _, err := os.Stat(d.cfg.DBPath); err != nil {
		fmt.Println("Graph not found. Run: codeGraphIndexer update graph --force")
		os.Exit(1)
	}

	var queryVector []float64
	var stored string
	err := d.sql.QueryRow(`
		SELECT e.vector FROM embeddings e
		JOIN symbols s ON s.id = e.symbol_id
		WHERE s.full_name = ? LIMIT 1`, term).Scan(&stored)
	if err == nil {
		_ = json.Unmarshal([]byte(stored), &queryVector)
	} else if err != sql.ErrNoRows {
		return err
	}
	if queryVector == nil {
		queryVector, err = client.EmbedOne(term)
		if err != nil {
			fmt.Printf("Failed to get embedding for: %s\n", term)
			os.Exit(1)
		}
	}
	qNorm := embed.L2Norm(queryVector)

	rows, err := d.sql.Query(`
		SELECT e.vector, e.norm, s.type, s.full_name, f.path, s.line
		FROM embeddings e
		JOIN symbols s ON s.id = e.symbol_id
		JOIN files f ON f.id = s.file_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	top := make([]similarHit, 0, 15)
	for rows.Next() {
		var vecJSON string
		var norm sql.NullFloat64
		var h similarHit
		if err := rows.Scan(&vecJSON, &norm, &h.typ, &h.full, &h.path, &h.line); err != nil {
			return err
		}
		var vec []float64
		if err := json.Unmarshal([]byte(vecJSON), &vec); err != nil || vec == nil {
			continue
		}
		n := norm.Float64
		if !norm.Valid || n == 0 {
			n = embed.L2Norm(vec)
		}
		h.sim = embed.Cosine(queryVector, vec, qNorm, n)
		if len(top) < 15 {
			top = append(top, h)
			sort.Slice(top, func(i, j int) bool { return top[i].sim > top[j].sim })
		} else if h.sim > top[len(top)-1].sim {
			top[len(top)-1] = h
			sort.Slice(top, func(i, j int) bool { return top[i].sim > top[j].sim })
		}
	}

	fmt.Printf("Similar to: %q\n\n", term)
	for _, h := range top {
		fmt.Printf("%.4f  %-12s %-50s %s:%d\n", h.sim, h.typ, h.full, h.path, h.line)
	}
	return nil
}
