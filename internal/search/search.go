package search

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codegraphindexer/internal/config"
	"codegraphindexer/internal/ignore"
)

func stripStringsAndComments(line string) string {
	re := regexp.MustCompile("`[^`]*`|\"[^\"]*\"|'[^']*'")
	line = re.ReplaceAllString(line, `""`)
	re2 := regexp.MustCompile(`//.*$`)
	return re2.ReplaceAllString(line, "")
}

func bracketDepth(line string) int {
	clean := stripStringsAndComments(line)
	return strings.Count(clean, "{") - strings.Count(clean, "}")
}

func Method(cfg *config.Config, rel, methodName string) error {
	filePath := config.AbsPath(cfg.RootDir, rel)
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("File not found: %s\n", rel)
		os.Exit(1)
	}
	if methodName == "" {
		fmt.Println("Method name required")
		os.Exit(1)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")

	mn := regexp.QuoteMeta(methodName)
	var startPatterns []*regexp.Regexp
	switch ext {
	case "php":
		startPatterns = []*regexp.Regexp{
			regexp.MustCompile(`function\s+` + mn + `\s*\(`),
		}
	case "rs":
		startPatterns = []*regexp.Regexp{
			regexp.MustCompile(`\bfn\s+` + mn + `\s*[<(]`),
		}
	case "go":
		startPatterns = []*regexp.Regexp{
			regexp.MustCompile(`^func\s+\(\w+\s+\*?\w+\)\s+` + mn + `\s*\(`),
			regexp.MustCompile(`^func\s+` + mn + `\s*[(\[]`),
		}
	default:
		startPatterns = []*regexp.Regexp{
			regexp.MustCompile(`function\s+` + mn + `\s*\(`),
			regexp.MustCompile(`^\s*(` + mn + `)\s*[=:]\s*(async\s+)?(function|\()`),
			regexp.MustCompile(`^\s*(?:async\s+)?(` + mn + `)\s*\([^)]*\)\s*\{`),
			regexp.MustCompile(`^\s*(` + mn + `)\s*=\s*(?:async\s+)?\(`),
		}
	}

	startLine := 0
	for i, line := range lines {
		clean := stripStringsAndComments(line)
		for _, p := range startPatterns {
			if p.MatchString(clean) && strings.Contains(clean, methodName) {
				startLine = i + 1
				break
			}
		}
		if startLine > 0 {
			break
		}
	}
	if startLine == 0 {
		fmt.Printf("Method not found: %s\n", methodName)
		os.Exit(1)
	}

	contextStart := startLine - 3
	if contextStart < 1 {
		contextStart = 1
	}
	fmt.Printf("=== context (lines %d-%d) ===\n", contextStart, startLine-1)
	for i := contextStart - 1; i < startLine-1; i++ {
		fmt.Print(lines[i])
	}

	depth := 0
	endLine := startLine
	started := false
	for i := startLine - 1; i < len(lines); i++ {
		depth += bracketDepth(lines[i])
		if !started && depth > 0 {
			started = true
		}
		if started && depth <= 0 {
			endLine = i + 1
			break
		}
	}
	fmt.Printf("=== method (lines %d-%d) ===\n", startLine, endLine)
	for i := startLine - 1; i < endLine; i++ {
		fmt.Print(lines[i])
	}
	return nil
}

func Outline(cfg *config.Config, rel string) error {
	filePath := config.AbsPath(cfg.RootDir, rel)
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("File not found: %s\n", rel)
		os.Exit(1)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")

	if ext == "scss" || ext == "css" {
		depth := 0
		re := regexp.MustCompile(`^\s*([^/\*@][^{]+)\{`)
		for i, line := range lines {
			open := strings.Count(line, "{")
			close := strings.Count(line, "}")
			if depth == 0 && open > 0 {
				if m := re.FindStringSubmatch(line); m != nil {
					fmt.Printf("%-50s line %d\n", strings.TrimSpace(m[1]), i+1)
				}
			}
			depth += open - close
		}
		return nil
	}

	jsKeywords := map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"else": true, "return": true, "typeof": true, "instanceof": true,
		"new": true, "delete": true, "void": true, "throw": true, "async": true,
	}
	var patterns []*regexp.Regexp
	switch ext {
	case "php":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`(?:public|protected|private|static|\s)+function\s+(\w+)\s*\(`),
		}
	case "rs":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:(?:default|const|async|unsafe|extern(?:\s+"[^"]*")?)\s+)*fn\s+(\w+)`),
		}
	case "go":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`^func\s+\(\w+\s+\*?(\w+)\)\s+(\w+)\s*\(`),
			regexp.MustCompile(`^func\s+(\w+)\s*[(\[]`),
		}
	default:
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`^\s*(?:async\s+)?([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\([^)]*\)\s*[{=]`),
			regexp.MustCompile(`(?:const|let|var)\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=\s*(?:async\s+)?(?:function|\()`),
			regexp.MustCompile(`^\s*([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=\s*(?:async\s+)?\(`),
			regexp.MustCompile(`^\s*(?:export\s+(?:default\s+)?)?(?:async\s+)?function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`),
		}
	}

	seen := map[int]bool{}
	for i, line := range lines {
		clean := stripStringsAndComments(line)
		for _, p := range patterns {
			if m := p.FindStringSubmatch(clean); m != nil {
				if seen[i] {
					break
				}
				name := m[len(m)-1]
				if ext == "go" && len(m) == 3 {
					name = m[1] + "." + m[2]
				}
				if jsKeywords[name] {
					break
				}
				seen[i] = true
				fmt.Printf("%-40s line %d\n", name+"()", i+1)
				break
			}
		}
	}
	return nil
}

func SCSS(cfg *config.Config, rel, selector string) error {
	filePath := config.AbsPath(cfg.RootDir, rel)
	selector = strings.TrimSpace(selector)
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("File not found: %s\n", rel)
		os.Exit(1)
	}
	if selector == "" {
		fmt.Println("Selector required")
		os.Exit(1)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	needle := regexp.QuoteMeta(strings.TrimLeft(selector, ".#&"))
	re1 := regexp.MustCompile(`[.#&\s]?` + needle + `\s*[{,&]`)
	re2 := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*[{,]`)

	startLine := 0
	for i, line := range lines {
		if re1.MatchString(line) || re2.MatchString(line) {
			startLine = i + 1
			break
		}
	}
	if startLine == 0 {
		fmt.Printf("Selector not found: %s\n", selector)
		os.Exit(1)
	}
	depth := 0
	endLine := startLine
	started := false
	for i := startLine - 1; i < len(lines); i++ {
		open := strings.Count(lines[i], "{")
		close := strings.Count(lines[i], "}")
		depth += open - close
		if !started && open > 0 {
			started = true
		}
		if started && depth <= 0 {
			endLine = i + 1
			break
		}
	}
	fmt.Printf("=== %s (%s) lines %d-%d ===\n", rel, selector, startLine, endLine)
	for i := startLine - 1; i < endLine; i++ {
		fmt.Print(lines[i])
	}
	return nil
}

func Entity(cfg *config.Config, rel string) error {
	filePath := config.AbsPath(cfg.RootDir, rel)
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("File not found: %s\n", rel)
		os.Exit(1)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	inConstructor := false
	depth := 0
	reField := regexp.MustCompile(`^\s*(public|protected|private|readonly).+?\$(\w+)`)
	reCtor := regexp.MustCompile(`function\s+__construct\s*\(`)

	for i, line := range lines {
		if !inConstructor && reField.MatchString(line) {
			fmt.Printf("%d: %s\n", i+1, strings.TrimSpace(line))
			continue
		}
		if reCtor.MatchString(line) {
			inConstructor = true
		}
		if inConstructor {
			fmt.Printf("%d: %s\n", i+1, strings.TrimSpace(line))
			depth += bracketDepth(line)
			if depth == 0 && strings.Contains(line, "}") {
				break
			}
		}
	}
	return nil
}

func Context(cfg *config.Config, rel string, center, radius int) error {
	filePath := config.AbsPath(cfg.RootDir, rel)
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("File not found: %s\n", rel)
		os.Exit(1)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	from := center - radius - 1
	if from < 0 {
		from = 0
	}
	to := center + radius - 1
	if to >= len(lines) {
		to = len(lines) - 1
	}
	fmt.Printf("=== %s lines %d-%d ===\n", rel, from+1, to+1)
	for i := from; i <= to; i++ {
		marker := "   "
		if i+1 == center {
			marker = ">>>"
		}
		fmt.Printf("%s %4d: %s", marker, i+1, lines[i])
	}
	return nil
}

func Route(cfg *config.Config, term string) error {
	if _, err := os.Stat(cfg.RouteFile); err != nil {
		fmt.Println("GetRoute.php not found")
		os.Exit(1)
	}
	data, err := os.ReadFile(cfg.RouteFile)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(term)) {
			fmt.Printf("%d: %s\n", i+1, strings.TrimSpace(line))
		}
	}
	return nil
}

func SQL(cfg *config.Config, term string) error {
	patterns := ignore.Load(cfg.RootDir)
	reSQL := regexp.MustCompile(`(?i)SELECT|INSERT|UPDATE|DELETE|FROM|INTO|JOIN|CREATE|ALTER`)
	reCont := regexp.MustCompile(`(?i)WHERE|AND|OR|LEFT|RIGHT|INNER|ON|GROUP|ORDER|LIMIT|SET|VALUES|\)|\(`)

	for _, dir := range cfg.SQLDirs {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		_ = ignore.WalkFiltered(dir, cfg.RootDir, patterns, func(abs, rel string) error {
			ext := strings.TrimPrefix(filepath.Ext(abs), ".")
			if ext != "php" && ext != "go" && ext != "rs" && ext != "sql" {
				return nil
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			inSQL := false
			sqlStart := 0
			var sqlBuf strings.Builder
			for i, line := range lines {
				isSQLLine := reSQL.MatchString(line)
				hasTable := strings.Contains(strings.ToLower(line), strings.ToLower(term))
				if isSQLLine && !inSQL {
					inSQL = true
					sqlStart = i + 1
					sqlBuf.Reset()
				}
				if inSQL {
					sqlBuf.WriteString(strings.TrimSpace(line))
					sqlBuf.WriteByte(' ')
					if !isSQLLine && !reCont.MatchString(line) {
						if strings.Contains(strings.ToLower(sqlBuf.String()), strings.ToLower(term)) {
							fmt.Printf("%s:%d: %s\n", rel, sqlStart, strings.TrimSpace(sqlBuf.String()))
						}
						inSQL = false
						sqlBuf.Reset()
					}
				} else if hasTable {
					fmt.Printf("%s:%d: %s\n", rel, i+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
	}
	return nil
}

func TextSearch(cfg *config.Config, action, term string) error {
	var patterns []*regexp.Regexp
	switch action {
	case "usages":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(term + `\s*\(`),
			regexp.MustCompile(`::` + term + `\b`),
			regexp.MustCompile(`\b` + term + `\b`),
		}
	case "class":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`class\s+` + term + `\b`),
			regexp.MustCompile(`new\s+` + term + `\b`),
			regexp.MustCompile(`use\s+[\\a-zA-Z\\]*` + term + `\b`),
		}
	case "extends":
		patterns = []*regexp.Regexp{regexp.MustCompile(`extends\s+` + term + `\b`)}
	case "implements":
		patterns = []*regexp.Regexp{regexp.MustCompile(`implements\s+[a-zA-Z,\s]*` + term + `\b`)}
	case "import":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`import\s+.*` + term),
			regexp.MustCompile(`require\s*\(.*` + term),
		}
	default:
		patterns = []*regexp.Regexp{regexp.MustCompile(regexp.QuoteMeta(term))}
	}

	ign := ignore.Load(cfg.RootDir)
	extOK := map[string]bool{}
	for _, e := range cfg.Extensions {
		extOK[e] = true
	}
	seen := map[string]bool{}
	var results []string

	for _, dir := range cfg.SearchDirs {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		_ = ignore.WalkFiltered(dir, cfg.RootDir, ign, func(abs, rel string) error {
			ext := strings.TrimPrefix(filepath.Ext(abs), ".")
			if !extOK[ext] {
				return nil
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				for _, p := range patterns {
					if p.MatchString(line) {
						msg := fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line))
						if !seen[msg] {
							seen[msg] = true
							results = append(results, msg)
						}
						break
					}
				}
			}
			return nil
		})
	}
	if len(results) == 0 {
		fmt.Printf("Not found: %s\n", term)
	} else {
		fmt.Println(strings.Join(results, "\n"))
		fmt.Printf("\n(%d matches)\n", len(results))
	}
	return nil
}
