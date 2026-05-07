package codeindex

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Symbol represents a named definition found in source code.
type Symbol struct {
	Name      string
	File      string // relative to repo root
	Line      int
	Signature string
	Preview   string // first 5 lines of body
}

// Match is a single line hit from a text search.
type Match struct {
	File    string // relative to repo root
	Line    int
	Content string // trimmed line text
}

// FindSymbol locates symbols matching query (exact name first, then partial).
// Returns up to 20 results.
func FindSymbol(repoPath, query string) ([]Symbol, error) {
	queryLow := strings.ToLower(query)
	var exact, partial []Symbol

	err := walkCode(repoPath, func(path string) {
		syms, _ := extractSymbols(path)
		for _, s := range syms {
			nameLow := strings.ToLower(s.Name)
			rel, _ := filepath.Rel(repoPath, s.File)
			s.File = rel
			if nameLow == queryLow {
				exact = append(exact, s)
			} else if strings.Contains(nameLow, queryLow) {
				partial = append(partial, s)
			}
		}
	})
	if err != nil {
		return nil, err
	}

	results := append(exact, partial...)
	if len(results) > 20 {
		results = results[:20]
	}
	return results, nil
}

// Search does case-insensitive full-text search across code files.
func Search(repoPath, query, lang string, limit int) ([]Match, error) {
	if limit <= 0 {
		limit = 20
	}
	queryLow := strings.ToLower(query)
	var results []Match

	filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error { //nolint:errcheck
		if err != nil {
			if path == repoPath {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if lang != "" && !matchesLang(path, lang) {
			return nil
		}
		if !isCodeFile(path) {
			return nil
		}
		matches, _ := grepFile(path, queryLow)
		for _, m := range matches {
			rel, _ := filepath.Rel(repoPath, m.File)
			m.File = rel
			results = append(results, m)
		}
		if len(results) >= limit {
			return filepath.SkipAll
		}
		return nil
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Callers finds all call sites of a named function.
func Callers(repoPath, funcName string, limit int) ([]Match, error) {
	return Search(repoPath, funcName+"(", "", limit)
}

// Deps returns the import list for a file or package directory.
// Supports Go and Python.
func Deps(repoPath, target string) ([]string, error) {
	abs := target
	if !filepath.IsAbs(target) {
		abs = filepath.Join(repoPath, target)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}

	var files []string
	if info.IsDir() {
		entries, _ := os.ReadDir(abs)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".py") {
				files = append(files, filepath.Join(abs, name))
			}
		}
	} else {
		files = []string{abs}
	}

	seen := map[string]bool{}
	var imports []string
	for _, f := range files {
		var imps []string
		switch {
		case strings.HasSuffix(f, ".go"):
			imps, _ = extractGoImports(f)
		case strings.HasSuffix(f, ".py"):
			imps, _ = extractPythonImports(f)
		}
		for _, imp := range imps {
			if !seen[imp] {
				seen[imp] = true
				imports = append(imports, imp)
			}
		}
	}
	return imports, nil
}

// ReadLines returns lines [start, end] (1-indexed, inclusive) from a file.
func ReadLines(path string, start, end int) (string, error) {
	lines, err := readLines(path)
	if err != nil {
		return "", err
	}
	if start < 1 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		return "", nil
	}
	return strings.Join(lines[start-1:end], "\n"), nil
}

// ── symbol extraction ─────────────────────────────────────────────────────────

func extractSymbols(path string) ([]Symbol, error) {
	switch {
	case strings.HasSuffix(path, ".go"):
		return extractGoSymbols(path)
	case strings.HasSuffix(path, ".py"):
		return extractPythonSymbols(path)
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"),
		strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".jsx"):
		return extractJSSymbols(path)
	}
	return nil, nil
}

func extractGoSymbols(path string) ([]Symbol, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, nil // skip files with syntax errors
	}

	lines, _ := readLines(path)
	var syms []Symbol

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			pos := fset.Position(d.Pos())
			syms = append(syms, Symbol{
				Name:      d.Name.Name,
				File:      path,
				Line:      pos.Line,
				Signature: buildFuncSig(d),
				Preview:   linesPreview(lines, pos.Line-1, 5),
			})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					pos := fset.Position(s.Pos())
					syms = append(syms, Symbol{
						Name:      s.Name.Name,
						File:      path,
						Line:      pos.Line,
						Signature: "type " + s.Name.Name + " " + typeStr(s.Type),
						Preview:   linesPreview(lines, pos.Line-1, 3),
					})
				case *ast.ValueSpec:
					for _, name := range s.Names {
						pos := fset.Position(name.Pos())
						tok := "var"
						if d.Tok == token.CONST {
							tok = "const"
						}
						syms = append(syms, Symbol{
							Name:      name.Name,
							File:      path,
							Line:      pos.Line,
							Signature: tok + " " + name.Name,
							Preview:   linesPreview(lines, pos.Line-1, 2),
						})
					}
				}
			}
		}
	}
	return syms, nil
}

func buildFuncSig(d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		recv := d.Recv.List[0]
		sb.WriteString("(")
		if len(recv.Names) > 0 {
			sb.WriteString(recv.Names[0].Name)
			sb.WriteString(" ")
		}
		sb.WriteString(typeStr(recv.Type))
		sb.WriteString(") ")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString("(")
	if d.Type.Params != nil {
		for i, p := range d.Type.Params.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			for j, n := range p.Names {
				if j > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(n.Name)
			}
			if len(p.Names) > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(typeStr(p.Type))
		}
	}
	sb.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		sb.WriteString(" ")
		res := d.Type.Results.List
		if len(res) == 1 && len(res[0].Names) == 0 {
			sb.WriteString(typeStr(res[0].Type))
		} else {
			sb.WriteString("(")
			for i, r := range res {
				if i > 0 {
					sb.WriteString(", ")
				}
				for j, n := range r.Names {
					if j > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(n.Name)
					sb.WriteString(" ")
				}
				sb.WriteString(typeStr(r.Type))
			}
			sb.WriteString(")")
		}
	}
	return sb.String()
}

func typeStr(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeStr(t.X)
	case *ast.SelectorExpr:
		return typeStr(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeStr(t.Elt)
		}
		return "[n]" + typeStr(t.Elt)
	case *ast.MapType:
		return "map[" + typeStr(t.Key) + "]" + typeStr(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + typeStr(t.Elt)
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + typeStr(t.Value)
		case ast.RECV:
			return "<-chan " + typeStr(t.Value)
		default:
			return "chan " + typeStr(t.Value)
		}
	case *ast.FuncType:
		return "func(...)"
	case *ast.StructType:
		return "struct{...}"
	default:
		return "_"
	}
}

// top-level Python defs/classes only (no indentation)
var pySymRe = regexp.MustCompile(`^(def|class)\s+(\w+)`)

func extractPythonSymbols(path string) ([]Symbol, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var syms []Symbol
	for i, line := range lines {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		m := pySymRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		syms = append(syms, Symbol{
			Name:      m[2],
			File:      path,
			Line:      i + 1,
			Signature: strings.TrimRight(line, " :{"),
			Preview:   linesPreview(lines, i, 3),
		})
	}
	return syms, nil
}

var jsFuncRe = regexp.MustCompile(
	`(?:export\s+)?(?:async\s+)?(?:function\s+(\w+)|(?:const|let|var)\s+(\w+)\s*=|class\s+(\w+))`,
)

func extractJSSymbols(path string) ([]Symbol, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var syms []Symbol
	for i, line := range lines {
		m := jsFuncRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name == "" {
			name = m[3]
		}
		if name == "" {
			continue
		}
		syms = append(syms, Symbol{
			Name:      name,
			File:      path,
			Line:      i + 1,
			Signature: strings.TrimSpace(line),
			Preview:   linesPreview(lines, i, 3),
		})
	}
	return syms, nil
}

// ── import extraction ─────────────────────────────────────────────────────────

func extractGoImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, nil
	}
	var imps []string
	for _, imp := range f.Imports {
		imps = append(imps, strings.Trim(imp.Path.Value, `"`))
	}
	return imps, nil
}

var pyImportRe = regexp.MustCompile(`^(?:import\s+(\S+)|from\s+(\S+)\s+import)`)

func extractPythonImports(path string) ([]string, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var imps []string
	for _, line := range lines {
		m := pyImportRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		imp := m[1]
		if imp == "" {
			imp = m[2]
		}
		imps = append(imps, imp)
	}
	return imps, nil
}

// ── file helpers ──────────────────────────────────────────────────────────────

func grepFile(path, queryLow string) ([]Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []Match
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), queryLow) {
			matches = append(matches, Match{
				File:    path,
				Line:    lineNum,
				Content: strings.TrimSpace(line),
			})
		}
	}
	return matches, scanner.Err()
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func linesPreview(lines []string, startIdx, count int) string {
	if startIdx < 0 || startIdx >= len(lines) {
		return ""
	}
	end := startIdx + count
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[startIdx:end], "\n")
}

func walkCode(repoPath string, fn func(string)) error {
	return filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == repoPath {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		fn(path)
		return nil
	})
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules", "__pycache__", ".venv", "venv",
		"dist", "build", ".next", "target", ".cache", "coverage":
		return true
	}
	return false
}

func isCodeFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx",
		".java", ".c", ".cpp", ".h", ".rs", ".rb", ".sh", ".lua":
		return true
	}
	return false
}

func matchesLang(path, lang string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch strings.ToLower(lang) {
	case "go":
		return ext == ".go"
	case "python", "py":
		return ext == ".py"
	case "typescript", "ts":
		return ext == ".ts" || ext == ".tsx"
	case "javascript", "js":
		return ext == ".js" || ext == ".jsx"
	default:
		return ext == "."+strings.ToLower(lang)
	}
}
