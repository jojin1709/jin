package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Module is one Jin file: the file that was run, or a file another file
// imported.
type Module struct {
	// Name is how the file is named in errors — its path relative to the file
	// that was run — and is empty for that file itself, because a message about
	// the file you ran does not need to say so.
	Name string
	// Path is where the file lives, absolute. Empty for a line typed into the
	// REPL, which is not a file at all.
	Path string
	// Dir is what this file's own imports are resolved against.
	Dir string
	// Program is the parsed file.
	Program *Program
	// Imports are the imports this file wrote, in source order.
	Imports []*Import
	// Deps is every file this one reaches, directly or not, in an order where a
	// file always comes after the files it imports.
	Deps []*Module
	// Prefix keeps this file's top-level names apart from every other file's in
	// the one global table they all share. The file that was run keeps the
	// plain names.
	Prefix string
}

// Import is one import statement, resolved to the file it names.
type Import struct {
	Alias  string
	Module *Module
	Token  Token
}

// Aliases maps the names this file can reach other files by.
func (m *Module) Aliases() map[string]*Module {
	out := make(map[string]*Module, len(m.Imports))
	for _, imp := range m.Imports {
		out[imp.Alias] = imp.Module
	}
	return out
}

// defines reports whether the file declares a top-level name. An alias must
// not clash with one, because then the bare name would be ambiguous to read.
func (m *Module) defines(name string) bool {
	for _, stmt := range m.Program.Statements {
		switch node := stmt.(type) {
		case *LetStatement:
			if node.Name.Value == name {
				return true
			}
		case *FunctionStatement:
			if node.Name.Value == name {
				return true
			}
		case *StructStatement:
			if node.Name.Value == name {
				return true
			}
		}
	}
	return false
}

// readError says a file could not be read. It is kept apart from the other
// load failures so a missing file can be reported as exactly that.
type readError struct {
	path string
	err  error
}

func (e *readError) Error() string {
	where := filepath.ToSlash(filepath.Base(e.path))
	if errors.Is(e.err, fs.ErrNotExist) {
		return where + " is not there"
	}
	return fmt.Sprintf("cannot read %s: %v", where, e.err)
}

func (e *readError) Unwrap() error { return e.err }

// cycleError says one file reaches itself through its imports.
type cycleError struct{ chain []string }

func (e *cycleError) Error() string {
	return "import cycle: " + strings.Join(e.chain, " -> ")
}

// Loader reads files and remembers them, so a file imported from two places is
// only read and parsed once.
type Loader struct {
	cache    map[string]*Module
	stack    []string // files being read right now, for cycle reporting
	entryDir string   // what imported files are named relative to
	nextID   int
}

// NewLoader returns a loader with nothing cached.
func NewLoader() *Loader {
	return &Loader{cache: map[string]*Module{}}
}

// LoadFile loads a file and everything it imports. The file itself keeps the
// plain global names; everything it imports is namespaced behind its alias.
func (l *Loader) LoadFile(file string) (*Module, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %v", file, err)
	}
	l.entryDir = filepath.Dir(abs)
	return l.file(abs, true)
}

// LoadStatements wraps statements that are already parsed, which is what the
// REPL hands over. Imports are resolved against dir.
func (l *Loader) LoadStatements(program *Program, dir string) (*Module, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if l.entryDir == "" {
		l.entryDir = abs
	}
	m := &Module{Name: "", Dir: abs, Program: program}
	if err := l.resolveImports(m); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadProject loads the project rooted at dir: its manifest, then its entry
// file and everything that file imports.
func (l *Loader) LoadProject(dir string) (*Module, *Manifest, error) {
	manifest, err := LoadManifest(dir)
	if err != nil {
		return nil, nil, err
	}
	m, err := l.LoadFile(filepath.Join(dir, filepath.FromSlash(manifest.Entry)))
	if err != nil {
		var read *readError
		if errors.As(err, &read) && errors.Is(read.err, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("%s: the entry file %s is not there",
				manifestName, filepath.ToSlash(manifest.Entry))
		}
		return nil, nil, err
	}
	return m, manifest, nil
}

// file reads one file, once. isEntry is only true for the file that was run,
// which is the one that goes without a name and without a namespace.
func (l *Loader) file(abs string, isEntry bool) (*Module, error) {
	if m, ok := l.cache[abs]; ok {
		if i := l.onStack(abs); i >= 0 {
			return nil, &cycleError{chain: l.cycleChain(i, abs)}
		}
		return m, nil
	}

	source, err := os.ReadFile(abs)
	if err != nil {
		return nil, &readError{path: abs, err: err}
	}

	name := ""
	if !isEntry {
		name = l.relative(abs)
	}

	p := NewParser(lexerFor(string(source), name))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("%s has syntax errors: %s", l.display(abs, isEntry), strings.Join(errs, "; "))
	}

	m := &Module{Name: name, Path: abs, Dir: filepath.Dir(abs), Program: program}
	if !isEntry {
		m.Prefix = fmt.Sprintf("#%d#", l.nextID)
		l.nextID++
	}
	l.cache[abs] = m

	l.stack = append(l.stack, abs)
	err = l.resolveImports(m)
	l.stack = l.stack[:len(l.stack)-1]
	if err != nil {
		delete(l.cache, abs)
		return nil, err
	}
	return m, nil
}

// resolveImports reads every file this one imports, checking the import paths,
// the aliases and the way they fit together.
func (l *Loader) resolveImports(m *Module) error {
	seen := map[string]Token{}

	for _, stmt := range m.Program.Statements {
		imp, ok := stmt.(*ImportStatement)
		if !ok {
			continue
		}
		alias := imp.Alias.Value

		if err := checkImportPath(imp); err != nil {
			return fmt.Errorf("%s: %v", imp.Token.Position(), err)
		}
		if _, isBuiltin := builtinIndexMap[alias]; isBuiltin {
			return fmt.Errorf("%s: %q is a built-in function; pick another name for the import",
				imp.Token.Position(), alias)
		}
		if first, dup := seen[alias]; dup {
			return fmt.Errorf("%s: %q is already the name for another import (line %d)",
				imp.Token.Position(), alias, first.Line)
		}
		seen[alias] = imp.Token
		if m.defines(alias) {
			return fmt.Errorf("%s: this file also defines %q, so the name would be ambiguous",
				imp.Token.Position(), alias)
		}

		dep, err := l.file(filepath.Join(m.Dir, filepath.FromSlash(imp.Path))+".jin", false)
		if err != nil {
			return l.wrapImportError(imp, err)
		}
		m.Imports = append(m.Imports, &Import{Alias: alias, Module: dep, Token: imp.Token})
	}

	m.collectDeps()
	return nil
}

// wrapImportError points an import's failure at the import that caused it,
// except for a cycle, which already names the files involved.
func (l *Loader) wrapImportError(imp *ImportStatement, err error) error {
	var cycle *cycleError
	if errors.As(err, &cycle) {
		return err
	}

	var read *readError
	if errors.As(err, &read) {
		where := l.relative(read.path)
		if errors.Is(read.err, fs.ErrNotExist) {
			return fmt.Errorf("%s: cannot find module %q; there is no %s to import",
				imp.Token.Position(), imp.Path, where)
		}
		return fmt.Errorf("%s: cannot read module %q: %v", imp.Token.Position(), imp.Path, read.err)
	}

	return fmt.Errorf("%s: importing %q: %v", imp.Token.Position(), imp.Path, err)
}

// collectDeps flattens the imports into one order where a file always comes
// after everything it imports.
func (m *Module) collectDeps() {
	seen := map[*Module]bool{}

	var add func(mod *Module)
	add = func(mod *Module) {
		if seen[mod] {
			return
		}
		seen[mod] = true
		for _, imp := range mod.Imports {
			add(imp.Module)
		}
		m.Deps = append(m.Deps, mod)
	}

	for _, imp := range m.Imports {
		add(imp.Module)
	}
}

// relative names a file the way errors should: its path from the file that was
// run, so "lib/math.jin" reads the way the import that reached it does.
func (l *Loader) relative(abs string) string {
	if l.entryDir != "" {
		if rel, err := filepath.Rel(l.entryDir, abs); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filepath.Base(abs))
}

// display names a file in a message about loading it. The file that was run is
// called by its base name, because there is nothing to be relative to yet.
func (l *Loader) display(abs string, isEntry bool) string {
	if isEntry {
		return filepath.ToSlash(filepath.Base(abs))
	}
	return l.relative(abs)
}

func (l *Loader) onStack(abs string) int {
	for i, p := range l.stack {
		if p == abs {
			return i
		}
	}
	return -1
}

// cycleChain describes a cycle as the imports that lead back to the file.
func (l *Loader) cycleChain(from int, abs string) []string {
	chain := make([]string, 0, len(l.stack)-from+1)
	for _, p := range l.stack[from:] {
		chain = append(chain, l.relative(p))
	}
	return append(chain, l.relative(abs))
}

// lexerFor reads a file's text, tagging its tokens with the file's name unless
// it is the file that was run.
func lexerFor(source, name string) *Lexer {
	if name == "" {
		return NewLexer(source)
	}
	return NewLexerFile(source, name)
}

// checkImportPath makes sure an import names a file beside the importing one.
func checkImportPath(imp *ImportStatement) error {
	spec := imp.Path

	switch {
	case strings.TrimSpace(spec) == "":
		return errors.New(`an import needs a path, like import "utils" as u`)
	case strings.HasSuffix(spec, ".jin"):
		return fmt.Errorf("import %q: leave the .jin off, because a path names a file without it", spec)
	case strings.Contains(spec, `\`):
		return fmt.Errorf(`import %q: use / between folders`, spec)
	case strings.Contains(spec, ":"):
		return fmt.Errorf("import %q: an import path cannot contain ':'", spec)
	case path.IsAbs(spec) || filepath.IsAbs(spec):
		return fmt.Errorf("import %q: an import path is relative to this file", spec)
	}

	for _, seg := range strings.Split(spec, "/") {
		switch seg {
		case "":
			return fmt.Errorf("import %q: empty path segment", spec)
		case ".":
			return fmt.Errorf("import %q: drop the leading ./", spec)
		case "..":
			return fmt.Errorf("import %q: an import cannot point outside this file's folder", spec)
		}
	}
	return nil
}

// ---------------------------------------------------------------- manifest

// manifestName is the file that marks a folder as a project.
const manifestName = "jin.toml"

// Manifest is what a project's jin.toml says about it.
type Manifest struct {
	Name    string
	Version string
	Entry   string // the file to run, relative to the project folder
	Dir     string // where jin.toml was read from
}

// Label names a project in messages, as its name and version.
func (m *Manifest) Label() string {
	return fmt.Sprintf("%s %s", m.Name, m.Version)
}

// LoadManifest reads the jin.toml in dir.
func LoadManifest(dir string) (*Manifest, error) {
	text, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, fmt.Errorf("%s: %v", manifestName, err)
	}

	m := &Manifest{
		Name:    filepath.Base(dir),
		Version: "0.1.0",
		Entry:   "main.jin",
		Dir:     dir,
	}

	for i, line := range strings.Split(string(text), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected name = \"value\"", manifestName, i+1)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)

		switch key {
		case "name":
			if value != "" {
				m.Name = value
			}
		case "version":
			if value != "" {
				m.Version = value
			}
		case "entry":
			if value != "" {
				m.Entry = value
			}
		default:
			return nil, fmt.Errorf("%s:%d: unknown setting %q", manifestName, i+1, key)
		}
	}

	if err := checkEntryPath(m.Entry); err != nil {
		return nil, fmt.Errorf("%s: %v", manifestName, err)
	}
	return m, nil
}

// checkEntryPath makes sure a manifest points at a file in its own folder.
func checkEntryPath(entry string) error {
	switch {
	case strings.TrimSpace(entry) == "":
		return errors.New(`entry must name a file, like entry = "main.jin"`)
	case !strings.HasSuffix(entry, ".jin"):
		return fmt.Errorf("entry %q: a Jin file ends in .jin", entry)
	case strings.Contains(entry, `\`):
		return fmt.Errorf("entry %q: use / between folders", entry)
	case strings.Contains(entry, ":"):
		return fmt.Errorf("entry %q: a path cannot contain ':'", entry)
	case path.IsAbs(entry) || filepath.IsAbs(entry):
		return fmt.Errorf("entry %q: the entry file is inside the project folder", entry)
	}
	for _, seg := range strings.Split(entry, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("entry %q: the entry file is inside the project folder", entry)
		}
	}
	return nil
}

// FindProject walks up from dir looking for a jin.toml, so a command works
// from anywhere inside a project, the way a build tool does.
func FindProject(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, manifestName)); err == nil {
			return abs, true
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}

// ---------------------------------------------------------------- new project

// NewProject writes a starting project into dir. The name is the project's
// name in its manifest, and also the folder it goes in when dir is empty.
func NewProject(dir, name string) error {
	if !validProjectName(name) {
		return fmt.Errorf("%q is not a project name; use letters, digits, - and _", name)
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	files := map[string]string{
		manifestName: fmt.Sprintf(`# Settings for this Jin project.
name = %q
version = "0.1.0"
entry = "main.jin"
`, name),
		"main.jin": `# Run this with 'jin run' and check it with 'jin check'.

import "greeting" as greeting

let who = args()
if len(who) == 0 {
    print(greeting.hello("world"))
} else {
    print(greeting.hello(who[0]))
}
`,
		"greeting.jin": `# Names in this file are reached as greeting.<name>.

fn hello(who: str) -> str {
    return "Hello, {who}!"
}
`,
	}

	for file, content := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// validProjectName keeps a project name to what can also be a folder name.
func validProjectName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9', r == '-':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
