package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"codegraphindexer/internal/config"
	"codegraphindexer/internal/graph"
	mysqlcmd "codegraphindexer/internal/mysql"
	"codegraphindexer/internal/search"
)

func usage() {
	fmt.Println("Usage: codeGraphIndexer <action> <term>")
	fmt.Println("Actions: usages, class, extends, implements, import, raw, method, block, outline, entity, route, sql, context, scss, schema, db, graph, similar, update")
	fmt.Println("  update graph [--force]")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	action := os.Args[1]
	term := ""
	if len(os.Args) > 2 {
		term = os.Args[2]
	}
	term2 := ""
	if len(os.Args) > 3 {
		term2 = os.Args[3]
	}
	term3 := ""
	if len(os.Args) > 4 {
		term3 = os.Args[4]
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if action == "update" {
		if term != "graph" {
			fmt.Println("Usage: codeGraphIndexer update graph [--force]")
			os.Exit(1)
		}
		force := false
		for _, a := range os.Args[3:] {
			if a == "--force" {
				force = true
			}
		}
		db, err := graph.Open(cfg)
		must(err)
		defer db.Close()
		must(db.Update(force, false))
		return
	}

	needsTerm := map[string]bool{
		"usages": true, "class": true, "extends": true, "implements": true,
		"import": true, "raw": true, "method": true, "block": true,
		"outline": true, "entity": true, "route": true, "sql": true,
		"context": true, "scss": true, "schema": true, "db": true,
		"graph": true, "similar": true,
	}
	if needsTerm[action] && term == "" {
		usage()
		os.Exit(1)
	}

	var gdb *graph.DB
	if action == "graph" || action == "similar" {
		gdb, err = graph.Open(cfg)
		must(err)
		defer gdb.Close()
		if err := gdb.Update(false, true); err != nil {
			fmt.Fprintf(os.Stderr, "update graph: %v\n", err)
			os.Exit(1)
		}
	}

	switch action {
	case "method", "block":
		must(search.Method(cfg, term, term2))
	case "outline":
		must(search.Outline(cfg, term))
	case "scss":
		must(search.SCSS(cfg, term, term2))
	case "entity":
		must(search.Entity(cfg, term))
	case "context":
		center, _ := strconv.Atoi(term2)
		radius := 10
		if term3 != "" {
			radius, _ = strconv.Atoi(term3)
		}
		if center == 0 {
			center = 1
		}
		must(search.Context(cfg, term, center, radius))
	case "route":
		must(search.Route(cfg, term))
	case "sql":
		must(search.SQL(cfg, term))
	case "schema":
		if err := mysqlcmd.Schema(cfg, term); err != nil {
			fmt.Println(err.Error())
		}
	case "db":
		if err := mysqlcmd.Query(cfg, term); err != nil {
			fmt.Println(err.Error())
		}
	case "similar":
		must(gdb.Similar(term))
	case "graph":
		if term2 == "" {
			fmt.Println("Usage: codeGraphIndexer graph <subaction> <name>")
			fmt.Println("Subactions: usages, methods, callers, deps, chain, files")
			os.Exit(1)
		}
		switch term {
		case "usages":
			must(gdb.GraphUsages(term2))
		case "methods":
			must(gdb.GraphMethods(term2))
		case "callers":
			must(gdb.GraphCallers(term2))
		case "deps":
			must(gdb.GraphDeps(term2))
		case "chain":
			must(gdb.GraphChain(term2))
		case "files":
			must(gdb.GraphFiles(term2))
		default:
			fmt.Printf("Unknown subaction: %s\n", term)
			fmt.Println("Subactions: usages, methods, callers, deps, chain, files")
			os.Exit(1)
		}
	case "usages", "class", "extends", "implements", "import", "raw":
		must(search.TextSearch(cfg, action, term))
	default:
		fmt.Printf("Unknown action: %s\n", action)
		usage()
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		msg := err.Error()
		if strings.TrimSpace(msg) != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		os.Exit(1)
	}
}
