package parse

import (
	"regexp"
	"strings"
)

var jsKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"return": true, "typeof": true, "new": true, "delete": true, "void": true,
	"throw": true, "else": true, "export": true, "import": true, "const": true,
	"let": true, "var": true, "class": true, "function": true,
}

var jsCallKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"function": true, "class": true, "return": true, "typeof": true, "new": true,
	"delete": true, "void": true, "throw": true,
}

var (
	reJSImportNamed = regexp.MustCompile(`import\s+\{([^}]+)\}`)
	reJSImportDef   = regexp.MustCompile(`import\s+(\w+)`)
	reJSImportLine  = regexp.MustCompile(`^import\s+(?:\{[^}]+\}|(\w+)|\*\s+as\s+\w+)(?:,\s*(?:\{[^}]+\}|(\w+)))?\s+from`)
	reJSClass       = regexp.MustCompile(`(?:export\s+default\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?`)
	reJSExportFn    = regexp.MustCompile(`^export\s+(?:default\s+)?(?:async\s+)?function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`)
	reJSFn          = regexp.MustCompile(`(?:async\s+)?(?:function\s+)?([a-zA-Z_$][a-zA-Z0-9_$]*)\s*[=:]?\s*(?:async\s+)?(?:function\s*)?\(`)
	reJSFnRaw       = regexp.MustCompile(`^\s*(?:async\s+)?(?:(?:const|let|var)\s+)?([a-zA-Z_$][a-zA-Z0-9_$]*)\s*[=(]`)
	reJSX           = regexp.MustCompile(`<([A-Z][a-zA-Z0-9]*)[\s/>]`)
	reJSCall        = regexp.MustCompile(`(?:this\.)?([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`)
)

func ParseJS(content, relPath string) Result {
	var r Result
	lines := strings.Split(content, "\n")
	currentClass := ""
	currentMethod := ""

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		lineNum := i + 1
		exportedFn := false

		if reJSImportLine.MatchString(line) {
			if named := reJSImportNamed.FindStringSubmatch(line); named != nil {
				for _, n := range strings.Split(named[1], ",") {
					n = strings.TrimSpace(n)
					if n != "" {
						r.addRef(relPath, n, "import", lineNum)
					}
				}
			}
			if def := reJSImportDef.FindStringSubmatch(line); def != nil && def[1] != "React" {
				// Avoid treating "import { named }" as default import of "{"
				if !strings.HasPrefix(strings.TrimPrefix(line, "import "), "{") {
					r.addRef(relPath, def[1], "import", lineNum)
				}
			}
		}

		if m := reJSClass.FindStringSubmatch(line); m != nil {
			currentClass = m[1]
			currentMethod = ""
			r.addSymbol("component", currentClass, currentClass, lineNum, "", 0)
			if m[2] != "" {
				r.addRef(currentClass, m[2], "extends", lineNum)
			}
		}

		if m := reJSExportFn.FindStringSubmatch(line); m != nil {
			currentMethod = m[1]
			fullName := m[1]
			if currentClass != "" {
				fullName = currentClass + "::" + m[1]
			}
			r.addSymbol("function", m[1], fullName, lineNum, "", 0)
			exportedFn = true
		}

		// General function/method — skip if already recorded as export function.
		if !exportedFn {
			if m := reJSFn.FindStringSubmatch(line); m != nil {
				name := m[1]
				if !jsKeywords[name] && reJSFnRaw.MatchString(raw) {
					currentMethod = name
					fullName := name
					if currentClass != "" {
						fullName = currentClass + "::" + name
					}
					r.addSymbol("function", name, fullName, lineNum, "", 0)
				}
			}
		}

		context := relPath
		if currentClass != "" {
			if currentMethod != "" {
				context = currentClass + "::" + currentMethod
			} else {
				context = currentClass
			}
		}

		for _, m := range reJSX.FindAllStringSubmatch(line, -1) {
			r.addRef(context, m[1], "jsx", lineNum)
		}
		for _, m := range reJSCall.FindAllStringSubmatch(line, -1) {
			if !jsCallKeywords[m[1]] {
				r.addRef(context, m[1], "call", lineNum)
			}
		}
	}
	return r
}

func ParseAstro(content, relPath string) Result {
	lines := strings.Split(content, "\n")
	fmCount := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			lines[i] = ""
			fmCount++
			if fmCount >= 2 {
				break
			}
		}
	}
	return ParseJS(strings.Join(lines, "\n"), relPath)
}
