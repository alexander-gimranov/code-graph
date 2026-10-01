package mysqlcmd

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"codegraphindexer/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

var tableNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func open(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true&multiStatements=false",
		cfg.MySQL.User, cfg.MySQL.Pass, cfg.MySQL.Host, cfg.MySQL.Name)
	return sql.Open("mysql", dsn)
}

func Schema(cfg *config.Config, table string) error {
	if !tableNameRe.MatchString(table) {
		return fmt.Errorf("invalid table name: %s", table)
	}
	db, err := open(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query("DESCRIBE `" + table + "`")
	if err != nil {
		return fmt.Errorf("DB error: %w", err)
	}
	defer rows.Close()
	return printSQLTable(rows)
}

func Query(cfg *config.Config, query string) error {
	if !regexp.MustCompile(`(?i)^\s*SELECT\s`).MatchString(query) {
		fmt.Fprintln(os.Stdout, "Only SELECT queries allowed")
		os.Exit(1)
	}
	db, err := open(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("DB error: %w", err)
	}
	defer rows.Close()
	return printSQLTable(rows)
}

func printSQLTable(rows *sql.Rows) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	var data [][]string
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		row := make([]string, len(cols))
		for i, v := range raw {
			row[i] = fmt.Sprint(stringify(v))
		}
		data = append(data, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(data) == 0 {
		fmt.Println("No rows")
		return nil
	}
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = utf8.RuneCountInString(c)
	}
	for _, row := range data {
		for i, cell := range row {
			if n := utf8.RuneCountInString(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	sepParts := make([]string, len(widths))
	for i, w := range widths {
		sepParts[i] = strings.Repeat("-", w+2)
	}
	sep := "+" + strings.Join(sepParts, "+") + "+"
	fmt.Println(sep)
	var header strings.Builder
	for i, c := range cols {
		header.WriteString("| ")
		header.WriteString(pad(c, widths[i]))
		header.WriteByte(' ')
	}
	header.WriteByte('|')
	fmt.Println(header.String())
	fmt.Println(sep)
	for _, row := range data {
		var line strings.Builder
		for i, cell := range row {
			line.WriteString("| ")
			line.WriteString(pad(cell, widths[i]))
			line.WriteByte(' ')
		}
		line.WriteByte('|')
		fmt.Println(line.String())
	}
	fmt.Println(sep)
	fmt.Printf("%d rows\n", len(data))
	return nil
}

func stringify(v any) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func pad(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}
