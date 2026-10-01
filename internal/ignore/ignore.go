package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var ignoreFiles = []string{
	".agentignore",
	".claudeignore",
	".cursorignore",
	".aiderignore",
	".copilotignore",
}

// Load reads ignore patterns from project root. Defaults always include node_modules, vendor, .git.
func Load(rootDir string) []string {
	patterns := []string{"node_modules", "vendor", ".git"}
	root := strings.TrimRight(rootDir, `\/`)
	seen := map[string]bool{"node_modules": true, "vendor": true, ".git": true}

	for _, name := range ignoreFiles {
		path := filepath.Join(root, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || line[0] == '#' || line[0] == '!' {
				continue
			}
			if !seen[line] {
				seen[line] = true
				patterns = append(patterns, line)
			}
		}
		f.Close()
	}
	return patterns
}

// IsIgnored reports whether relPath (slash-separated) matches any pattern.
func IsIgnored(relPath string, patterns []string) bool {
	relPath = strings.Trim(strings.ReplaceAll(relPath, `\`, `/`), "/")
	parts := strings.Split(relPath, "/")

	for _, raw := range patterns {
		pattern := strings.Trim(strings.ReplaceAll(raw, `\`, `/`), "/")
		if pattern == "" {
			continue
		}
		hasWild := strings.ContainsAny(pattern, "*?")
		hasSlash := strings.Contains(pattern, "/")

		switch {
		case !hasWild && !hasSlash:
			for _, p := range parts {
				if p == pattern {
					return true
				}
			}
		case hasWild:
			if match(pattern, relPath) {
				return true
			}
			for _, p := range parts {
				if match(pattern, p) {
					return true
				}
			}
		default:
			if strings.HasPrefix(relPath+"/", pattern+"/") || strings.HasPrefix(relPath, pattern+"/") {
				return true
			}
		}
	}
	return false
}

func match(pattern, name string) bool {
	ok, err := doublestar.Match(pattern, name)
	return err == nil && ok
}

// WalkFiltered walks dir and calls fn for each regular file not ignored.
func WalkFiltered(dir, rootDir string, patterns []string, fn func(absPath, relPath string) error) error {
	rootReal := filepath.Clean(strings.TrimRight(rootDir, `\/`))
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		abs := path
		if r, e := filepath.Abs(path); e == nil {
			abs = r
		}
		rel, e := filepath.Rel(rootReal, abs)
		if e != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)
		if IsIgnored(rel, patterns) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		return fn(abs, rel)
	})
}
