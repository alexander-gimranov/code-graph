package parse

import (
	"regexp"
	"strings"
)

var (
	rePHPNamespace = regexp.MustCompile(`^namespace\s+([\w\\]+)`)
	rePHPClass     = regexp.MustCompile(`^(?:abstract\s+|final\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+([\w,\s]+))?`)
	rePHPInterface = regexp.MustCompile(`^interface\s+(\w+)(?:\s+extends\s+([\w,\s]+))?`)
	rePHPMethod    = regexp.MustCompile(`^(public|protected|private)?\s*(static\s+)?function\s+(\w+)\s*\(`)
	rePHPNew       = regexp.MustCompile(`new\s+([A-Z]\w+)\s*[(\[]`)
	rePHPStatic    = regexp.MustCompile(`([A-Z]\w+)::(\w+)`)
	rePHPCall      = regexp.MustCompile(`->(\w+)\s*\(`)
	rePHPUse       = regexp.MustCompile(`^use\s+([\w\\]+)`)
)

func ParsePHP(content, relPath string) Result {
	var r Result
	lines := strings.Split(content, "\n")
	currentClass := ""
	currentMethod := ""

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		lineNum := i + 1

		if m := rePHPNamespace.FindStringSubmatch(line); m != nil {
			_ = m[1]
		}

		if m := rePHPClass.FindStringSubmatch(line); m != nil {
			currentClass = m[1]
			currentMethod = ""
			r.addSymbol("class", currentClass, currentClass, lineNum, "", 0)
			if m[2] != "" {
				r.addRef(currentClass, strings.TrimSpace(m[2]), "extends", lineNum)
			}
			if m[3] != "" {
				for _, iface := range strings.Split(m[3], ",") {
					iface = strings.TrimSpace(iface)
					if iface != "" {
						r.addRef(currentClass, iface, "implements", lineNum)
					}
				}
			}
		}

		if m := rePHPInterface.FindStringSubmatch(line); m != nil {
			currentClass = m[1]
			r.addSymbol("class", currentClass, currentClass, lineNum, "", 0)
			if m[2] != "" {
				for _, parent := range strings.Split(m[2], ",") {
					parent = strings.TrimSpace(parent)
					if parent != "" {
						r.addRef(currentClass, parent, "extends", lineNum)
					}
				}
			}
		}

		if m := rePHPMethod.FindStringSubmatch(line); m != nil {
			vis := m[1]
			if vis == "" {
				vis = "public"
			}
			isStatic := 0
			if m[2] != "" {
				isStatic = 1
			}
			currentMethod = m[3]
			fullName := currentMethod
			if currentClass != "" {
				fullName = currentClass + "::" + currentMethod
			}
			r.addSymbol("method", currentMethod, fullName, lineNum, vis, isStatic)
		}

		context := relPath
		if currentClass != "" {
			if currentMethod != "" {
				context = currentClass + "::" + currentMethod
			} else {
				context = currentClass
			}
		}

		for _, m := range rePHPNew.FindAllStringSubmatch(line, -1) {
			r.addRef(context, m[1], "instantiate", lineNum)
		}
		for _, m := range rePHPStatic.FindAllStringSubmatch(line, -1) {
			if m[1] == "self" || m[1] == "parent" || m[1] == "static" {
				continue
			}
			r.addRef(context, m[1]+"::"+m[2], "static_call", lineNum)
		}
		for _, m := range rePHPCall.FindAllStringSubmatch(line, -1) {
			r.addRef(context, m[1], "call", lineNum)
		}
		if m := rePHPUse.FindStringSubmatch(line); m != nil {
			parts := strings.Split(m[1], `\`)
			r.addRef(relPath, parts[len(parts)-1], "import", lineNum)
		}
	}
	return r
}
