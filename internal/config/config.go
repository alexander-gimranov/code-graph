package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MySQL struct {
	Host string `json:"host"`
	Name string `json:"name"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

type Embed struct {
	Provider    string `json:"provider"`
	Key         string `json:"key"`
	OllamaURL   string `json:"ollamaURL"`
	OllamaModel string `json:"ollamaModel"`
}

type Config struct {
	RootDir    string
	ToolDir    string
	MySQL      MySQL               `json:"mysql"`
	DBPath     string              `json:"dbPath"`
	ScanDirs   map[string][]string `json:"scanDirs"`
	ScanExts   map[string][]string `json:"scanExts"`
	SQLDirs    []string            `json:"sqlDirs"`
	SearchDirs []string            `json:"searchDirs"`
	Extensions []string            `json:"extensions"`
	RouteFile  string              `json:"routeFile"`
	Embed      Embed               `json:"embed"`
}

func Load() (*Config, error) {
	cfgPath, toolDir, err := findConfig()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config json: %w", err)
	}

	c.ToolDir = toolDir
	root := filepath.Dir(toolDir)
	c.RootDir = root + string(filepath.Separator)

	// dbPath is relative to tool dir (codeGraphIndexer/), not repo root.
	c.DBPath = resolveFrom(toolDir, c.DBPath)
	c.RouteFile = resolveFrom(root, c.RouteFile)
	c.ScanDirs = resolveMap(root, c.ScanDirs)
	c.SQLDirs = resolveSlice(root, c.SQLDirs)
	c.SearchDirs = resolveSlice(root, c.SearchDirs)

	if c.ScanExts == nil {
		c.ScanExts = map[string][]string{}
	}
	for lang := range c.ScanDirs {
		if _, ok := c.ScanExts[lang]; !ok {
			c.ScanExts[lang] = []string{lang}
		}
	}

	return &c, nil
}

func findConfig() (cfgPath, toolDir string, err error) {
	var candidates []string

	if exe, e := os.Executable(); e == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, "config.json"))
	}
	if wd, e := os.Getwd(); e == nil {
		candidates = append(candidates,
			filepath.Join(wd, "config.json"),
			filepath.Join(wd, "codeGraphIndexer", "config.json"),
		)
	}

	for _, p := range candidates {
		if st, e := os.Stat(p); e == nil && !st.IsDir() {
			abs, _ := filepath.Abs(p)
			return abs, filepath.Dir(abs), nil
		}
	}
	return "", "", fmt.Errorf("config.json not found (looked next to binary and in cwd)")
}

func resolveFrom(base, p string) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, filepath.FromSlash(p))
}

func resolveSlice(root string, paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = resolveFrom(root, p)
	}
	return out
}

func resolveMap(root string, m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, paths := range m {
		out[k] = resolveSlice(root, paths)
	}
	return out
}

// RelPath returns path relative to root with forward slashes.
func RelPath(abs, rootDir string) string {
	root := filepath.Clean(strings.TrimRight(rootDir, `\/`))
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		rel = abs
	}
	return filepath.ToSlash(rel)
}

// AbsPath joins root with a relative slash-path.
func AbsPath(rootDir, rel string) string {
	return filepath.Join(strings.TrimRight(rootDir, `\/`), filepath.FromSlash(rel))
}
