package parse

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	skipCalls = map[string]bool{
		"if": true, "while": true, "for": true, "match": true, "loop": true,
		"return": true, "let": true, "in": true, "as": true, "fn": true, "move": true,
		"else": true, "where": true, "impl": true, "pub": true, "mod": true, "use": true,
		"struct": true, "enum": true, "trait": true, "type": true, "const": true,
		"static": true, "unsafe": true, "async": true, "await": true, "dyn": true,
		"ref": true, "mut": true, "break": true, "continue": true, "crate": true,
		"self": true, "super": true, "box": true, "union": true, "extern": true,
	}
	skipMacros = map[string]bool{
		"println": true, "print": true, "eprintln": true, "eprint": true, "format": true,
		"write": true, "writeln": true, "vec": true, "assert": true, "assert_eq": true,
		"assert_ne": true, "debug_assert": true, "debug_assert_eq": true, "debug_assert_ne": true,
		"panic": true, "todo": true, "unimplemented": true, "unreachable": true, "matches": true,
		"dbg": true, "concat": true, "stringify": true, "include_str": true, "include_bytes": true,
		"env": true, "option_env": true, "cfg": true, "line": true, "file": true, "column": true,
		"format_args": true, "thread_local": true, "compile_error": true, "json": true, "vec_deque": true,
	}

	reRustAttr     = regexp.MustCompile(`^#!?\[`)
	reRustUse      = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?use\s+(.*)$`)
	reRustImplHead = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:unsafe\s+)?(?:impl|trait)\b`)
	reRustImpl     = regexp.MustCompile(`^(?:unsafe\s+)?impl\b(.*)$`)
	reRustTrait    = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:(?:default|const|async|unsafe|extern(?:\s+"[^"]*")?)\s+)*trait\s+(\w+)(.*)$`)
	reRustStruct   = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:(?:default|const|async|unsafe|extern(?:\s+"[^"]*")?)\s+)*(?:struct|enum|union)\s+(\w+)`)
	reRustMod      = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:(?:default|const|async|unsafe|extern(?:\s+"[^"]*")?)\s+)*mod\s+(\w+)`)
	reRustFn       = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:(?:default|const|async|unsafe|extern(?:\s+"[^"]*")?)\s+)*fn\s+(\w+)`)
	reRustVis      = regexp.MustCompile(`^(pub(?:\s*\([^)]*\))?\s+)?`)
	reRustConst    = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:(?:default|const|async|unsafe|extern(?:\s+"[^"]*")?)\s+)*(?:const|static)\s+(?:mut\s+)?(\w+)\s*:`)
	reRustType     = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:(?:default|const|async|unsafe|extern(?:\s+"[^"]*")?)\s+)*type\s+(\w+)`)
	reRustImplFor  = regexp.MustCompile(`^(!?)(.+?)\s+for\s+(.+)$`)
	reRustTraitExt = regexp.MustCompile(`^:\s*([^{]*?)(?:\swhere\s.*)?\s*(?:\{.*)?$`)
	reRustMacro    = regexp.MustCompile(`\b([A-Za-z_]\w*)!\s*[(\[{]`)
	reRustInst     = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])((?:\w+::)*)([A-Z]\w*)\s*\{`)
	reRustStatic   = regexp.MustCompile(`\b([A-Z]\w*)(?:::<[^>]*>)?::(\w+)\s*(?:::<[^>]*>)?\s*\(`)
	reRustMethod   = regexp.MustCompile(`\.\s*([a-z_]\w*)\s*(?:::<[^()]*>)?\s*\(`)
	reRustFnCall   = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])(?:[a-z_]\w*::)*([a-z_]\w*)\s*(?:::<[^()]*>)?\s*\(`)
	reRustTypeName = regexp.MustCompile(`^((?:\w+\s*::\s*)*)(\w+)`)
	reRustSelf     = regexp.MustCompile(`\bself\b`)
	reRustCharLit  = regexp.MustCompile(`'(?:\\[^']*|[^\\'])'`)
)

type rustFrame struct {
	depth int
	kind  string
	name  string
	ctx   string
}

func rustSanitize(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	n := len(src)
	i := 0
	for i < n {
		// Jump to next interesting character.
		j := i
		for j < n {
			c := src[j]
			if c == '/' || c == '"' || c == '\'' || c == 'r' || c == 'b' {
				break
			}
			j++
		}
		if j > i {
			out.WriteString(src[i:j])
			i = j
			if i >= n {
				break
			}
		}

		c := src[i]
		c2 := byte(0)
		if i+1 < n {
			c2 = src[i+1]
		}

		if c == '/' && c2 == '/' {
			for i < n && src[i] != '\n' {
				out.WriteByte(' ')
				i++
			}
			continue
		}
		if c == '/' && c2 == '*' {
			depth := 0
			for i < n {
				if i+1 < n && src[i] == '/' && src[i+1] == '*' {
					depth++
					out.WriteString("  ")
					i += 2
					continue
				}
				if i+1 < n && src[i] == '*' && src[i+1] == '/' {
					depth--
					out.WriteString("  ")
					i += 2
					if depth == 0 {
						break
					}
					continue
				}
				if src[i] == '\n' {
					out.WriteByte('\n')
				} else {
					out.WriteByte(' ')
				}
				i++
			}
			continue
		}

		// raw strings: r"..", r#".."#, br#".."#
		if (c == 'r' || (c == 'b' && c2 == 'r')) && (i == 0 || !isWordByte(src[i-1])) {
			start := i
			j := i
			if c == 'b' {
				j += 2
			} else {
				j++
			}
			h := 0
			for j < n && src[j] == '#' {
				h++
				j++
			}
			if j < n && src[j] == '"' {
				close := `"` + strings.Repeat("#", h)
				end := strings.Index(src[j+1:], close)
				if end < 0 {
					end = n
				} else {
					end = j + 1 + end + len(close)
				}
				out.WriteByte('"')
				chunk := src[start+1 : end-1]
				for _, ch := range chunk {
					if ch == '\n' {
						out.WriteByte('\n')
					} else {
						out.WriteByte(' ')
					}
				}
				out.WriteByte('"')
				i = end
				continue
			}
		}

		if c == '"' {
			out.WriteByte('"')
			i++
			for i < n && src[i] != '"' {
				if src[i] == '\\' && i+1 < n {
					out.WriteByte(' ')
					i++
				}
				if src[i] == '\n' {
					out.WriteByte('\n')
				} else {
					out.WriteByte(' ')
				}
				i++
			}
			out.WriteByte('"')
			i++
			continue
		}

		if c == '\'' {
			if m := reRustCharLit.FindStringIndex(src[i:]); m != nil && m[0] == 0 {
				lit := src[i : i+m[1]]
				out.WriteByte('\'')
				out.WriteString(strings.Repeat(" ", len(lit)-2))
				out.WriteByte('\'')
				i += len(lit)
				continue
			}
		}

		out.WriteByte(c)
		i++
	}
	return out.String()
}

func isWordByte(b byte) bool {
	return b == '_' || unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b))
}

func rustStripGenerics(s string) string {
	s = strings.TrimLeft(s, " \t")
	if s == "" || s[0] != '<' {
		return s
	}
	d := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			d++
		} else if s[i] == '>' {
			prev := byte(0)
			if i > 0 {
				prev = s[i-1]
			}
			if prev != '-' {
				d--
				if d == 0 {
					return strings.TrimLeft(s[i+1:], " \t")
				}
			}
		}
	}
	return ""
}

func rustTypeName(s string) string {
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`^[\s&*]*(?:mut\s+|const\s+|dyn\s+|'\w+\s+)*`)
	s = re.ReplaceAllString(s, "")
	if m := reRustTypeName.FindStringSubmatch(s); m != nil {
		return m[2]
	}
	return ""
}

func rustVisibility(vis string) string {
	vis = strings.TrimSpace(vis)
	if vis == "" {
		return "private"
	}
	if vis == "pub" {
		return "public"
	}
	return "protected"
}

func rustImplType(stack []rustFrame) string {
	for k := len(stack) - 1; k >= 0; k-- {
		if stack[k].kind == "impl" || stack[k].kind == "trait" {
			return stack[k].name
		}
	}
	return ""
}

func rustContext(stack []rustFrame, relPath string) string {
	for k := len(stack) - 1; k >= 0; k-- {
		if stack[k].kind == "fn" {
			return stack[k].ctx
		}
	}
	for k := len(stack) - 1; k >= 0; k-- {
		if (stack[k].kind == "impl" || stack[k].kind == "trait" || stack[k].kind == "struct") && stack[k].name != "" {
			return stack[k].name
		}
	}
	return relPath
}

func rustEmitUse(r *Result, relPath, text string, lineNum int) {
	text = regexp.MustCompile(`;.*$`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\s+as\s+\w+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(\w+)\s*::\s*\*`).ReplaceAllString(text, "$1,")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, "")
	// Emit leaf identifiers that appear before ',', '}', or end (path segments after ::).
	seen := map[string]bool{}
	for i := 0; i < len(text); {
		if !isIdentStart(text[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(text) && isIdentCont(text[j]) {
			j++
		}
		leaf := text[i:j]
		i = j
		// Skip if followed by '::' (not a leaf).
		if i+1 < len(text) && text[i] == ':' && text[i+1] == ':' {
			continue
		}
		if leaf == "self" || leaf == "super" || leaf == "crate" || seen[leaf] {
			continue
		}
		seen[leaf] = true
		r.addRef(relPath, leaf, "import", lineNum)
	}
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isIdentCont(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

func rustEmitRefs(r *Result, ctx, self, line string, lineNum int) {
	for _, m := range reRustMacro.FindAllStringSubmatch(line, -1) {
		if !skipMacros[m[1]] {
			r.addRef(ctx, m[1], "call", lineNum)
		}
	}
	for _, m := range reRustInst.FindAllStringSubmatch(line, -1) {
		t := m[2]
		if t == "Self" {
			t = self
		}
		if t != "" {
			r.addRef(ctx, t, "instantiate", lineNum)
		}
	}
	for _, m := range reRustStatic.FindAllStringSubmatch(line, -1) {
		t := m[1]
		if t == "Self" {
			t = self
		}
		if t != "" {
			r.addRef(ctx, t+"::"+m[2], "static_call", lineNum)
		}
	}
	for _, m := range reRustMethod.FindAllStringSubmatch(line, -1) {
		r.addRef(ctx, m[1], "call", lineNum)
	}
	for _, m := range reRustFnCall.FindAllStringSubmatch(line, -1) {
		if !skipCalls[m[1]] {
			r.addRef(ctx, m[1], "call", lineNum)
		}
	}
}

func ParseRust(content, relPath string) Result {
	var r Result
	lines := strings.Split(rustSanitize(content), "\n")

	var stack []rustFrame
	var pending *rustFrame
	depth := 0
	attrDeep := 0
	var useBuf *struct {
		line int
		text string
	}
	var headBuf *struct {
		line int
		text string
	}

	for i, raw := range lines {
		lineNum := i + 1
		line := strings.TrimSpace(raw)
		skipRefs := false
		var declared *rustFrame

		if attrDeep > 0 || reRustAttr.MatchString(line) {
			attrDeep += strings.Count(line, "[") - strings.Count(line, "]")
			if attrDeep < 0 {
				attrDeep = 0
			}
			continue
		}

		if useBuf != nil {
			useBuf.text += " " + line
			if strings.Contains(line, ";") {
				rustEmitUse(&r, relPath, useBuf.text, useBuf.line)
				useBuf = nil
			}
			continue
		}
		if m := reRustUse.FindStringSubmatch(line); m != nil {
			if strings.Contains(line, ";") {
				rustEmitUse(&r, relPath, m[1], lineNum)
			} else {
				useBuf = &struct {
					line int
					text string
				}{lineNum, m[1]}
			}
			continue
		}

		if headBuf != nil {
			headBuf.text += " " + line
			if !strings.Contains(line, "{") && !strings.Contains(line, ";") {
				continue
			}
			line = headBuf.text
			lineNum = headBuf.line
			headBuf = nil
		} else if reRustImplHead.MatchString(line) && !strings.Contains(line, "{") && !strings.Contains(line, ";") {
			headBuf = &struct {
				line int
				text string
			}{lineNum, line}
			continue
		}

		if m := reRustImpl.FindStringSubmatch(line); m != nil {
			head := rustStripGenerics(m[1])
			whereParts := regexp.MustCompile(`\swhere\s`).Split(head, 2)
			head = strings.TrimSpace(regexp.MustCompile(`\{.*$`).ReplaceAllString(whereParts[0], ""))
			typeName := ""
			if hm := reRustImplFor.FindStringSubmatch(head); hm != nil {
				typeName = rustTypeName(hm[3])
				trait := rustTypeName(hm[2])
				if trait != "" && typeName != "" {
					r.addRef(typeName, trait, "implements", lineNum)
				}
			} else {
				typeName = rustTypeName(head)
			}
			declared = &rustFrame{kind: "impl", name: typeName, ctx: typeName}
			skipRefs = true
		} else if m := reRustTrait.FindStringSubmatch(line); m != nil {
			vis := ""
			if vm := reRustVis.FindStringSubmatch(line); vm != nil {
				vis = vm[1]
			}
			r.addSymbol("class", m[1], m[1], lineNum, rustVisibility(vis), 0)
			rest := rustStripGenerics(m[2])
			if sm := reRustTraitExt.FindStringSubmatch(rest); sm != nil {
				for _, super := range strings.Split(sm[1], "+") {
					sn := rustTypeName(super)
					if sn != "" && sn[0] != '\'' {
						r.addRef(m[1], sn, "extends", lineNum)
					}
				}
			}
			declared = &rustFrame{kind: "trait", name: m[1], ctx: m[1]}
			skipRefs = true
		} else if m := reRustStruct.FindStringSubmatch(line); m != nil {
			vis := ""
			if vm := reRustVis.FindStringSubmatch(line); vm != nil {
				vis = vm[1]
			}
			r.addSymbol("class", m[1], m[1], lineNum, rustVisibility(vis), 0)
			declared = &rustFrame{kind: "struct", name: m[1], ctx: m[1]}
			skipRefs = true
		} else if reRustMod.MatchString(line) {
			declared = &rustFrame{kind: "mod", name: "", ctx: ""}
			skipRefs = true
		} else if m := reRustFn.FindStringSubmatch(line); m != nil {
			name := m[1]
			owner := rustImplType(stack)
			inFn := false
			for _, f := range stack {
				if f.kind == "fn" {
					inFn = true
					break
				}
			}
			topKind := ""
			if len(stack) > 0 {
				topKind = stack[len(stack)-1].kind
			}
			visM := reRustVis.FindStringSubmatch(line)
			vis := ""
			if visM != nil {
				vis = visM[1]
			}
			visStr := rustVisibility(vis)
			if topKind == "trait" {
				visStr = "public"
			}
			ctx := name
			if owner != "" && (topKind == "impl" || topKind == "trait") {
				full := owner + "::" + name
				params := ""
				if idx := strings.Index(line, "("); idx >= 0 {
					params = line[idx:]
				}
				isStatic := 0
				if params != "" {
					close := strings.Index(params, ")")
					if close >= 0 && !reRustSelf.MatchString(params[:close+1]) {
						isStatic = 1
					}
				}
				r.addSymbol("method", name, full, lineNum, visStr, isStatic)
				ctx = full
			} else {
				v := visStr
				if inFn {
					v = "private"
				}
				r.addSymbol("function", name, name, lineNum, v, 0)
			}
			declared = &rustFrame{kind: "fn", name: name, ctx: ctx}
			skipRefs = true
		} else if m := reRustConst.FindStringSubmatch(line); m != nil && m[1] != "_" {
			owner := rustImplType(stack)
			full := m[1]
			if owner != "" {
				full = owner + "::" + m[1]
			}
			vis := ""
			if vm := reRustVis.FindStringSubmatch(line); vm != nil {
				vis = vm[1]
			}
			r.addSymbol("property", m[1], full, lineNum, rustVisibility(vis), 0)
		} else if m := reRustType.FindStringSubmatch(line); m != nil && rustImplType(stack) == "" {
			vis := ""
			if vm := reRustVis.FindStringSubmatch(line); vm != nil {
				vis = vm[1]
			}
			r.addSymbol("class", m[1], m[1], lineNum, rustVisibility(vis), 0)
			skipRefs = true
		}

		top := ""
		if len(stack) > 0 {
			top = stack[len(stack)-1].kind
		}
		if !skipRefs && top != "struct" {
			ctx := rustContext(stack, relPath)
			self := rustImplType(stack)
			rustEmitRefs(&r, ctx, self, line, lineNum)
		} else if declared != nil && declared.kind == "fn" {
			if p := strings.Index(line, "{"); p >= 0 {
				rustEmitRefs(&r, declared.ctx, rustImplType(stack), line[p+1:], lineNum)
			}
		}

		if declared != nil {
			pending = declared
		}
		for k := 0; k < len(line); k++ {
			ch := line[k]
			if ch == '{' {
				if pending != nil {
					pending.depth = depth
					stack = append(stack, *pending)
					pending = nil
				}
				depth++
			} else if ch == '}' {
				depth--
				for len(stack) > 0 && depth <= stack[len(stack)-1].depth {
					stack = stack[:len(stack)-1]
				}
			}
		}
		if pending != nil && strings.Count(line, "(") <= strings.Count(line, ")") && strings.Contains(line, ";") {
			pending = nil
		}
	}
	return r
}
