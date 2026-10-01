package parse

import (
	"regexp"
	"strings"
)

var goKeywords = map[string]bool{
	"if": true, "for": true, "switch": true, "select": true, "case": true,
	"return": true, "defer": true, "go": true, "func": true, "range": true,
	"make": true, "new": true, "append": true, "len": true, "cap": true,
	"close": true, "delete": true, "copy": true, "panic": true, "recover": true,
	"print": true, "println": true, "var": true, "type": true, "map": true, "chan": true,
}

var (
	reGoImportSingle = regexp.MustCompile(`^import\s+"([\w.\-/]+)"`)
	reGoImportBlock  = regexp.MustCompile(`^\s*(?:\w+\s+)?"([\w.\-/]+)"`)
	reGoStruct       = regexp.MustCompile(`^type\s+(\w+)\s+struct\b`)
	reGoInterface    = regexp.MustCompile(`^type\s+(\w+)\s+interface\b`)
	reGoMethod       = regexp.MustCompile(`^func\s+\(\w+\s+\*?(\w+)\)\s+(\w+)\s*\(`)
	reGoFunc         = regexp.MustCompile(`^func\s+(\w+)\s*[(\[]`)
	reGoNew          = regexp.MustCompile(`&?([A-Z]\w+)\s*\{`)
	reGoDotCall      = regexp.MustCompile(`\.(\w+)\s*\(`)
	reGoCall         = regexp.MustCompile(`\b([a-z]\w*)\s*\(`)
)

func ParseGo(content, relPath string) Result {
	var r Result
	lines := strings.Split(content, "\n")
	currentType := ""
	currentMethod := ""

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		lineNum := i + 1

		if m := reGoImportSingle.FindStringSubmatch(line); m != nil {
			parts := strings.Split(m[1], "/")
			r.addRef(relPath, parts[len(parts)-1], "import", lineNum)
			continue
		}
		if m := reGoImportBlock.FindStringSubmatch(line); m != nil && !strings.Contains(line, "import") {
			parts := strings.Split(m[1], "/")
			r.addRef(relPath, parts[len(parts)-1], "import", lineNum)
		}

		if m := reGoStruct.FindStringSubmatch(line); m != nil {
			currentType = m[1]
			currentMethod = ""
			r.addSymbol("class", currentType, currentType, lineNum, "public", 0)
			continue
		}
		if m := reGoInterface.FindStringSubmatch(line); m != nil {
			currentType = m[1]
			currentMethod = ""
			r.addSymbol("class", currentType, currentType, lineNum, "public", 0)
			continue
		}
		if m := reGoMethod.FindStringSubmatch(line); m != nil {
			currentType = m[1]
			currentMethod = m[2]
			full := currentType + "::" + currentMethod
			r.addSymbol("method", currentMethod, full, lineNum, "public", 0)
			continue
		}
		if m := reGoFunc.FindStringSubmatch(line); m != nil {
			currentType = ""
			currentMethod = m[1]
			r.addSymbol("function", currentMethod, currentMethod, lineNum, "public", 0)
			continue
		}

		context := relPath
		if currentType != "" {
			if currentMethod != "" {
				context = currentType + "::" + currentMethod
			} else {
				context = currentType
			}
		} else if currentMethod != "" {
			context = currentMethod
		}

		for _, m := range reGoNew.FindAllStringSubmatch(line, -1) {
			r.addRef(context, m[1], "instantiate", lineNum)
		}
		for _, m := range reGoDotCall.FindAllStringSubmatch(line, -1) {
			r.addRef(context, m[1], "call", lineNum)
		}
		for _, m := range reGoCall.FindAllStringSubmatch(line, -1) {
			if !goKeywords[m[1]] {
				r.addRef(context, m[1], "call", lineNum)
			}
		}
	}
	return r
}
