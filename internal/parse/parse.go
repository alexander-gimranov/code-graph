package parse

// Symbol is a code symbol extracted by a parser.
type Symbol struct {
	Type       string
	Name       string
	FullName   string
	Line       int
	Visibility string
	IsStatic   int
}

// Ref is a code reference extracted by a parser.
type Ref struct {
	FromFullName string
	ToName       string
	RefType      string
	Line         int
}

// Result holds symbols and refs for one file.
type Result struct {
	Symbols []Symbol
	Refs    []Ref
}

func (r *Result) addSymbol(typ, name, fullName string, line int, visibility string, isStatic int) {
	r.Symbols = append(r.Symbols, Symbol{
		Type: typ, Name: name, FullName: fullName,
		Line: line, Visibility: visibility, IsStatic: isStatic,
	})
}

func (r *Result) addRef(from, to, refType string, line int) {
	if to == "" {
		return
	}
	r.Refs = append(r.Refs, Ref{
		FromFullName: from, ToName: to, RefType: refType, Line: line,
	})
}

// ParserFunc parses file content into symbols and refs.
type ParserFunc func(content, relPath string) Result

// Registry maps language keys to parser functions.
var Registry = map[string]ParserFunc{
	"php":   ParsePHP,
	"js":    ParseJS,
	"go":    ParseGo,
	"astro": ParseAstro,
	"rust":  ParseRust,
}
