package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------- helpers

// writeFiles writes each file into a fresh temporary folder, making any
// folders the names need, and returns the folder.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// loadEntry loads a file and everything it imports, failing the test on any
// loading problem.
func loadEntry(t *testing.T, path string) *Module {
	t.Helper()

	m, err := NewLoader().LoadFile(path)
	if err != nil {
		t.Fatalf("loading %s: %v", path, err)
	}
	return m
}

// runFileOnVM runs a file and its imports on the bytecode back end.
func runFileOnVM(t *testing.T, path string) (string, error) {
	t.Helper()

	m := loadEntry(t, path)
	bc, err := NewCompiler(NewSymbolTable(true)).CompileModules(m)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	vm := NewVM(&out)
	_, err = vm.Run(bc)
	return out.String(), err
}

// runFileOnInterpreter runs the same file on the tree-walking back end.
func runFileOnInterpreter(t *testing.T, path string) (string, error) {
	t.Helper()

	m := loadEntry(t, path)

	var out bytes.Buffer
	interp := NewInterpreter(&out)
	_, err := interp.RunModules(m, NewEnvironment())
	return out.String(), err
}

// expectFileError checks that loading a file fails with a message containing
// wantSub.
func expectFileError(t *testing.T, path, wantSub string) {
	t.Helper()

	_, err := NewLoader().LoadFile(path)
	if err == nil {
		t.Fatalf("expected an error containing %q, got none", wantSub)
	}
	if !strings.Contains(err.Error(), wantSub) {
		t.Errorf("want an error containing %q, got: %v", wantSub, err)
	}
}

// expectProjectTypeError checks that checking a file and its imports reports a
// problem containing wantSub.
func expectProjectTypeError(t *testing.T, path, wantSub string) {
	t.Helper()

	errs := CheckProject(loadEntry(t, path))
	if len(errs) == 0 {
		t.Fatalf("expected a type error containing %q, got none", wantSub)
	}
	if joined := strings.Join(errs, "; "); !strings.Contains(joined, wantSub) {
		t.Errorf("want a type error containing %q, got: %s", wantSub, joined)
	}
}

// ---------------------------------------------------------------- parsing

func TestParseImport(t *testing.T) {
	p := NewParser(NewLexer(`import "lib/utils" as u`))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	if len(program.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(program.Statements))
	}
	imp, ok := program.Statements[0].(*ImportStatement)
	if !ok {
		t.Fatalf("got %T, want an ImportStatement", program.Statements[0])
	}
	if imp.Path != "lib/utils" {
		t.Errorf("path = %q, want %q", imp.Path, "lib/utils")
	}
	if imp.Alias == nil || imp.Alias.Value != "u" {
		t.Errorf("alias = %v, want u", imp.Alias)
	}
	if got, want := imp.String(), `import "lib/utils" as u`; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestImportNeedsAPathAndAName(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"no path", `import as u`},
		{"no as", `import "utils"`},
		{"no alias", `import "utils" as`},
		{"alias is not a name", `import "utils" as 1`},
		{"nothing at all", `import`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser(NewLexer(tc.source))
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Errorf("%q should not parse", tc.source)
			}
		})
	}
}

func TestImportIsTopLevelOnly(t *testing.T) {
	cases := []string{
		"if true {\n    import \"u\" as u\n}",
		"fn f() {\n    import \"u\" as u\n}",
		"while false {\n    import \"u\" as u\n}",
		"for i in range(1) {\n    import \"u\" as u\n}",
	}

	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			p := NewParser(NewLexer(source))
			p.ParseProgram()
			joined := strings.Join(p.Errors(), "; ")
			if !strings.Contains(joined, "only allowed at the top level") {
				t.Errorf("want a top-level-only error, got: %s", joined)
			}
		})
	}
}

// ---------------------------------------------------------------- paths

func TestImportPathsAreChecked(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"", "an import needs a path"},
		{"   ", "an import needs a path"},
		{"utils.jin", "leave the .jin off"},
		{"lib\\utils", "use / between folders"},
		{"C:/utils", "cannot contain ':'"},
		{"/utils", "relative to this file"},
		{"./utils", "drop the leading ./"},
		{"../utils", "cannot point outside this file's folder"},
		{"lib//utils", "empty path segment"},
		{"lib/./utils", "drop the leading ./"},
		{"lib/../utils", "cannot point outside this file's folder"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			dir := writeFiles(t, map[string]string{
				"main.jin": `import "` + tc.path + `" as u`,
			})
			expectFileError(t, filepath.Join(dir, "main.jin"), tc.want)
		})
	}
}

func TestImportPathPrefersForwardSlashes(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":     "import \"lib/math\" as math\nprint(math.double(4))",
		"lib/math.jin": "fn double(n: int) -> int {\n    return n * 2\n}",
	})

	got, err := runFileOnVM(t, filepath.Join(dir, "main.jin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "8\n" {
		t.Errorf("output = %q, want %q", got, "8\n")
	}
}

// ---------------------------------------------------------------- loading

func TestImportsLoadInDependencyOrder(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"a\" as a\nimport \"b\" as b",
		"a.jin":    "import \"b\" as b\nlet a = 1",
		"b.jin":    "let b = 2",
	})

	m := loadEntry(t, filepath.Join(dir, "main.jin"))

	names := make([]string, 0, len(m.Deps))
	for _, dep := range m.Deps {
		names = append(names, dep.Name)
	}
	if got, want := strings.Join(names, ","), "b.jin,a.jin"; got != want {
		t.Errorf("dependency order = %q, want %q", got, want)
	}
	if len(m.Aliases()) != 2 {
		t.Errorf("got %d aliases, want 2", len(m.Aliases()))
	}
}

func TestAModuleIsLoadedOnce(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"a\" as a\nimport \"shared\" as s1\nprint(s1.x)",
		"a.jin":    "import \"shared\" as s2",
		"shared.jin": "print(\"shared loaded\")\n" +
			"let x = \"value\"",
	})

	m := loadEntry(t, filepath.Join(dir, "main.jin"))
	if len(m.Deps) != 2 {
		t.Fatalf("got %d dependencies, want 2", len(m.Deps))
	}

	a := m.Aliases()["a"]
	shared := m.Aliases()["s1"]
	if a.Aliases()["s2"] != shared {
		t.Error("the same file loaded twice under two aliases should be one module")
	}
}

func TestAModuleBodyRunsOnceBeforeTheFileThatImportsIt(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"a\" as a\n" +
			"import \"shared\" as s1\n" +
			"import \"shared\" as s2\n" +
			"print(\"main\")\n" +
			"print(s1.x, s2.x)",
		"a.jin":      "import \"shared\" as s\nprint(\"a\")",
		"shared.jin": "print(\"shared\")\nlet x = 7",
	})

	want := "shared\na\nmain\n7 7\n"
	for _, run := range []struct {
		name string
		fn   func(*testing.T, string) (string, error)
	}{
		{"vm", runFileOnVM},
		{"interpreter", runFileOnInterpreter},
	} {
		t.Run(run.name, func(t *testing.T) {
			got, err := run.fn(t, filepath.Join(dir, "main.jin"))
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("output = %q, want %q", got, want)
			}
		})
	}
}

func TestAModuleCannotBeFound(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"nope\" as n",
	})
	expectFileError(t, filepath.Join(dir, "main.jin"),
		`cannot find module "nope"; there is no nope.jin to import`)
}

func TestImportsCannotFormACycle(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"a\" as a",
		"a.jin":    "import \"b\" as b",
		"b.jin":    "import \"a\" as a",
	})
	expectFileError(t, filepath.Join(dir, "main.jin"), "import cycle: a.jin -> b.jin -> a.jin")
}

func TestAFileCannotImportItself(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"self.jin": "import \"self\" as s",
	})
	expectFileError(t, filepath.Join(dir, "self.jin"), "import cycle: self.jin -> self.jin")
}

func TestAnAliasIsUsedOncePerFile(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"u\" as u\nimport \"v\" as u",
		"u.jin":    "let a = 1",
		"v.jin":    "let b = 2",
	})
	expectFileError(t, filepath.Join(dir, "main.jin"),
		`"u" is already the name for another import (line 1)`)
}

func TestAnAliasMayNotClashWithAnOwnName(t *testing.T) {
	for _, decl := range []string{"let u = 1", "fn u() { return 1 }", "struct u { x: int }"} {
		t.Run(decl, func(t *testing.T) {
			dir := writeFiles(t, map[string]string{
				"main.jin":  "import \"utils\" as u\n" + decl,
				"utils.jin": "let a = 1",
			})
			expectFileError(t, filepath.Join(dir, "main.jin"),
				`this file also defines "u", so the name would be ambiguous`)
		})
	}
}

func TestAnAliasMayNotShadowABuiltin(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":  "import \"utils\" as print",
		"utils.jin": "let a = 1",
	})
	expectFileError(t, filepath.Join(dir, "main.jin"),
		`"print" is a built-in function; pick another name for the import`)
}

func TestASyntaxErrorInAnImportedFileNamesThatFile(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"broken\" as b",
		"broken.jin": "fn f() {\n" +
			"    return 1 +\n" +
			"}",
	})
	expectFileError(t, filepath.Join(dir, "main.jin"), "broken.jin has syntax errors")
}

func TestImportsCannotReachOutOfTheFolder(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":   "import \"lib/m\" as m",
		"lib/m.jin":  "import \"../shared\" as s",
		"shared.jin": "let a = 1",
	})
	expectFileError(t, filepath.Join(dir, "main.jin"),
		"import \"../shared\": an import cannot point outside this file's folder")
}

func TestTheSameFileNameInTwoFoldersIsTwoModules(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"who\" as outer\n" +
			"import \"lib/math\" as math\n" +
			"print(outer.name(), math.name())",
		"who.jin":      "fn name() -> str { return \"outer\" }",
		"lib/math.jin": "import \"who\" as inner\nfn name() -> str { return \"inner \" + inner.name() }",
		"lib/who.jin":  "fn name() -> str { return \"lib\" }",
	})

	want := "outer inner lib\n"
	got, err := runFileOnVM(t, filepath.Join(dir, "main.jin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------- checking

func TestAModulesNamesAreTypedThroughItsAlias(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":  "import \"utils\" as u\nlet x: int = u.double(2)",
		"utils.jin": "fn double(n: int) -> int {\n    return n * 2\n}",
	})
	if errs := CheckProject(loadEntry(t, filepath.Join(dir, "main.jin"))); len(errs) > 0 {
		t.Errorf("expected no type errors, got: %s", strings.Join(errs, "; "))
	}
}

func TestCallsIntoAModuleAreChecked(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":  "import \"utils\" as u\nprint(u.double())",
		"utils.jin": "fn double(n: int) -> int {\n    return n * 2\n}",
	})
	expectProjectTypeError(t, filepath.Join(dir, "main.jin"), "u.double() expects 1 argument(s), got 0")
}

func TestAModuleWithNoSuchNameIsCaught(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":  "import \"utils\" as u\nprint(u.helper)",
		"utils.jin": "fn double(n: int) -> int {\n    return n * 2\n}",
	})
	expectProjectTypeError(t, filepath.Join(dir, "main.jin"), `u has no name "helper"`)
}

func TestAModuleIsNotAValue(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":  "import \"utils\" as u\nprint(u)",
		"utils.jin": "fn double(n: int) -> int {\n    return n * 2\n}",
	})
	expectProjectTypeError(t, filepath.Join(dir, "main.jin"),
		`"u" is a module; write u.<name> to use a name from it`)
}

func TestAModulesNamesAreReadOnly(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":  "import \"utils\" as u\nu.helper = 1",
		"utils.jin": "fn helper() -> int {\n    return 1\n}",
	})
	expectProjectTypeError(t, filepath.Join(dir, "main.jin"),
		"cannot assign to u.helper; another file's names are read-only")
}

func TestTypeErrorsInAnImportedFileCarryItsName(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin":  "import \"utils\" as u\nprint(u.count)",
		"utils.jin": "let count: int = \"text\"",
	})
	expectProjectTypeError(t, filepath.Join(dir, "main.jin"), `utils.jin:1:1: cannot assign str to int variable "count"`)
}

func TestOnlyAModulesTopLevelNamesAreReachable(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"utils\" as u\nprint(u.hidden)",
		"utils.jin": "if true {\n" +
			"    let hidden = 1\n" +
			"}\n" +
			"let shown = 2",
	})
	expectProjectTypeError(t, filepath.Join(dir, "main.jin"), `u has no name "hidden"`)

	dir2 := writeFiles(t, map[string]string{
		"main.jin": "import \"utils\" as u\nprint(u.shown)",
		"utils.jin": "if true {\n" +
			"    let hidden = 1\n" +
			"}\n" +
			"let shown = 2",
	})
	if errs := CheckProject(loadEntry(t, filepath.Join(dir2, "main.jin"))); len(errs) > 0 {
		t.Errorf("a module's top-level name should be reachable, got: %s", strings.Join(errs, "; "))
	}
}

// ---------------------------------------------------------------- manifest

func TestManifestDefaults(t *testing.T) {
	dir := writeFiles(t, map[string]string{"jin.toml": "# nothing here\n"})

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Entry != "main.jin" {
		t.Errorf("entry = %q, want main.jin", m.Entry)
	}
	if m.Version != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", m.Version)
	}
	if m.Name != filepath.Base(dir) {
		t.Errorf("name = %q, want %q", m.Name, filepath.Base(dir))
	}
}

func TestManifestSettings(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"jin.toml": "name = \"demo\"\nversion = \"2.0.0\"\nentry = \"app.jin\"\n",
	})

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Label() != "demo 2.0.0" {
		t.Errorf("label = %q, want %q", m.Label(), "demo 2.0.0")
	}
	if m.Entry != "app.jin" {
		t.Errorf("entry = %q, want app.jin", m.Entry)
	}
}

func TestEmptyManifestValuesKeepTheDefaults(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"jin.toml": "name = \"\"\nversion = \"\"\nentry = \"\"\n",
	})

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Entry != "main.jin" || m.Version != "0.1.0" || m.Name != filepath.Base(dir) {
		t.Errorf("empty settings should keep the defaults, got %+v", m)
	}
}

func TestManifestRejectsBadSettings(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"unknown setting", "deps = \"x\"", `unknown setting "deps"`},
		{"not a setting", "name demo", "expected name = \"value\""},
		{"entry is not jin", "entry = \"app.txt\"", "a Jin file ends in .jin"},
		{"entry reaches out", "entry = \"../app.jin\"", "the entry file is inside the project folder"},
		{"entry uses backslashes", "entry = \"src\\\\app.jin\"", "use / between folders"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeFiles(t, map[string]string{"jin.toml": tc.content})
			_, err := LoadManifest(dir)
			if err == nil {
				t.Fatalf("expected an error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want an error containing %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestFindProjectWalksUp(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"jin.toml":       "name = \"demo\"",
		"main.jin":       "print(1)",
		"src/deep/x.jin": "print(2)",
	})

	found, ok := FindProject(filepath.Join(dir, "src", "deep"))
	if !ok {
		t.Fatal("expected to find the project")
	}
	if found != dir {
		t.Errorf("found %q, want %q", found, dir)
	}

	outside := t.TempDir()
	if _, ok := FindProject(outside); ok {
		t.Error("a folder with no project above it should not find one")
	}
}

func TestLoadProjectNeedsItsEntryFile(t *testing.T) {
	dir := writeFiles(t, map[string]string{"jin.toml": "name = \"demo\""})

	_, _, err := NewLoader().LoadProject(dir)
	if err == nil {
		t.Fatal("expected an error for a missing entry file")
	}
	if !strings.Contains(err.Error(), "the entry file main.jin is not there") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------- new project

func TestNewProjectWritesAStartingProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hello")

	if err := NewProject(dir, "hello"); err != nil {
		t.Fatal(err)
	}

	m, manifest, err := NewLoader().LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "hello" {
		t.Errorf("name = %q, want hello", manifest.Name)
	}
	if errs := CheckProject(m); len(errs) > 0 {
		t.Errorf("the starting project should type-check, got: %s", strings.Join(errs, "; "))
	}

	got, err := runFileOnVM(t, filepath.Join(dir, "main.jin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello, world!\n" {
		t.Errorf("output = %q, want %q", got, "Hello, world!\n")
	}
}

func TestNewProjectRefusesToOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hello")
	if err := NewProject(dir, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := NewProject(dir, "hello"); err == nil {
		t.Error("expected a refusal to write over an existing folder")
	}
}

func TestProjectNamesAreValidated(t *testing.T) {
	for _, name := range []string{"", "9bad", "with space", "with/slash", "with.dot"} {
		t.Run(name, func(t *testing.T) {
			if validProjectName(name) {
				t.Errorf("%q should not be a project name", name)
			}
		})
	}
	for _, name := range []string{"hello", "my_app", "my-app", "a1"} {
		t.Run(name, func(t *testing.T) {
			if !validProjectName(name) {
				t.Errorf("%q should be a project name", name)
			}
		})
	}
}

// ---------------------------------------------------------------- arguments

func TestArgsIsTheArgumentsTheProgramWasRunWith(t *testing.T) {
	source := `print(args())
print(len(args()))`

	for _, run := range []struct {
		name string
		fn   func(*testing.T, string, []string) (string, error)
	}{
		{"vm", runSourceOnVM},
		{"interpreter", runSourceOnInterpreter},
	} {
		t.Run(run.name, func(t *testing.T) {
			got, err := run.fn(t, source, []string{"one", "two"})
			if err != nil {
				t.Fatal(err)
			}
			if want := "[\"one\", \"two\"]\n2\n"; got != want {
				t.Errorf("output = %q, want %q", got, want)
			}

			none, err := run.fn(t, source, nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := "[]\n0\n"; none != want {
				t.Errorf("output with no arguments = %q, want %q", none, want)
			}
		})
	}
}

// runSourceOnVM runs one source string with the given program arguments.
func runSourceOnVM(t *testing.T, source string, programArgs []string) (string, error) {
	t.Helper()

	p := NewParser(NewLexer(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	bc, err := compileProgram(program)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	vm := NewVMWithArgs(&out, programArgs)
	_, err = vm.Run(bc)
	return out.String(), err
}

// runSourceOnInterpreter runs one source string with the given program
// arguments.
func runSourceOnInterpreter(t *testing.T, source string, programArgs []string) (string, error) {
	t.Helper()

	p := NewParser(NewLexer(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}

	var out bytes.Buffer
	interp := NewInterpreterWithArgs(&out, programArgs)
	_, err := interp.Run(program, NewEnvironment())
	return out.String(), err
}

func TestArgsIsAnArrayOfStrings(t *testing.T) {
	expectClean(t, `let a: array = args()`)
	expectClean(t, `let first: str = args()[0]`)
	expectClean(t, `for arg in args() { print(arg) }`)
	expectTypeError(t, `let a: int = args()`, "cannot assign array<str> to int")
	expectTypeError(t, `args(1)`, "args() takes 0 argument(s), got 1")
}

// ---------------------------------------------------------------- parity

// TestProjectsAgreeAcrossEngines runs whole projects on both back ends and
// requires identical output, the same guarantee the single-file corpus gives.
func TestProjectsAgreeAcrossEngines(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		entry string
		want  string
	}{
		{
			name: "a module's names behind an alias",
			files: map[string]string{
				"main.jin":  "import \"utils\" as u\nprint(u.double(21), u.name)",
				"utils.jin": "let name = \"utils\"\nfn double(n: int) -> int {\n    return n * 2\n}",
			},
			want: "42 utils\n",
		},
		{
			name: "a module that imports another module",
			files: map[string]string{
				"main.jin": "import \"a\" as a\nprint(a.twice(5))",
				"a.jin":    "import \"b\" as b\nfn twice(n: int) -> int {\n    return b.inc(b.inc(n))\n}",
				"b.jin":    "fn inc(n: int) -> int {\n    return n + 1\n}",
			},
			want: "7\n",
		},
		{
			name: "a module's variables are shared through it",
			files: map[string]string{
				"main.jin": "import \"counter\" as c\nc.bump()\nc.bump()\nprint(c.count)",
				"counter.jin": "let count = 0\n" +
					"fn bump() {\n    count = count + 1\n}",
			},
			want: "2\n",
		},
		{
			name: "two aliases for one file",
			files: map[string]string{
				"main.jin": "import \"u\" as a\nimport \"u\" as b\nprint(a.x + b.x)",
				"u.jin":    "let x = 20",
			},
			want: "40\n",
		},
		{
			name: "a module uses a closure it defined",
			files: map[string]string{
				"main.jin": "import \"mk\" as mk\nlet add = mk.adder(3)\nprint(add(4))",
				"mk.jin":   "fn adder(n: int) {\n    return fn(x: int) -> int { return x + n }\n}",
			},
			want: "7\n",
		},
		{
			name: "a module's function calls back into the file that imported it",
			files: map[string]string{
				"main.jin": "import \"call\" as c\nfn twice(n: int) -> int {\n    return n * 2\n}\nprint(c.apply(21))",
				"call.jin": "fn apply(n: int) -> int {\n    return twice(n)\n}",
			},
			want: "42\n",
		},
		{
			name: "a struct imported from a module",
			files: map[string]string{
				"main.jin": "import \"geo\" as geo\n" +
					"let p = geo.Point(10, 20)\n" +
					"p.x = p.x + 5\n" +
					"print(p, p.x, p.y, geo.scale(p, 2))",
				"geo.jin": "struct Point { x: int, y: int }\n" +
					"fn scale(p: Point, f: int) -> Point {\n" +
					"    return Point(p.x * f, p.y * f)\n" +
					"}",
			},
			want: "Point{x: 15, y: 20} 15 20 Point{x: 30, y: 40}\n",
		},
		{
			name: "a struct method imported from a module",
			files: map[string]string{
				"main.jin": "import \"geo\" as geo\n" +
					"let p = geo.Point(10, 20)\n" +
					"p.move(5, 5)\n" +
					"print(p, p.area())",
				"geo.jin": "struct Point { x: int, y: int }\n" +
					"fn (p: Point) move(dx: int, dy: int) {\n" +
					"    p.x = p.x + dx\n" +
					"    p.y = p.y + dy\n" +
					"}\n" +
					"fn (p: Point) area() -> int {\n" +
					"    return p.x * p.y\n" +
					"}",
			},
			want: "Point{x: 15, y: 25} 375\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeFiles(t, tc.files)
			entry := filepath.Join(dir, filepath.FromSlash(tc.entry))
			if tc.entry == "" {
				entry = filepath.Join(dir, "main.jin")
			}

			vmOut, vmErr := runFileOnVM(t, entry)
			interpOut, interpErr := runFileOnInterpreter(t, entry)

			if (vmErr == nil) != (interpErr == nil) {
				t.Fatalf("engines disagree on success:\n vm: %v\n interp: %v", vmErr, interpErr)
			}
			if vmErr != nil {
				if vmErr.Error() != interpErr.Error() {
					t.Fatalf("engines disagree on the error:\n vm: %v\n interp: %v", vmErr, interpErr)
				}
				return
			}
			if vmOut != interpOut {
				t.Fatalf("engines disagree on output:\n vm:\n%s\n interp:\n%s", vmOut, interpOut)
			}
			if vmOut != tc.want {
				t.Errorf("output = %q, want %q", vmOut, tc.want)
			}
		})
	}
}

// TestProjectRuntimeErrorsAgree checks the two back ends fail the same way,
// with the same position, when a module fails while it runs.
func TestProjectRuntimeErrorsAgree(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"u\" as u\nprint(\"before\")\nprint(u.boom())",
		"u.jin": "fn boom() -> int {\n" +
			"    let n = 0\n" +
			"    return 10 / n\n" +
			"}",
	})

	entry := filepath.Join(dir, "main.jin")
	vmOut, vmErr := runFileOnVM(t, entry)
	interpOut, interpErr := runFileOnInterpreter(t, entry)

	if vmErr == nil || interpErr == nil {
		t.Fatalf("expected both back ends to fail: vm=%v interp=%v", vmErr, interpErr)
	}
	if vmErr.Error() != interpErr.Error() {
		t.Errorf("engines disagree on the error:\n vm: %v\n interp: %v", vmErr, interpErr)
	}
	if vmOut != "before\n" || interpOut != "before\n" {
		t.Errorf("output before the failure = %q / %q, want %q", vmOut, interpOut, "before\n")
	}
	if want := "u.jin:3:15: division by zero"; vmErr.Error() != want {
		t.Errorf("error = %q, want %q", vmErr.Error(), want)
	}
}

// TestAFailingModuleStopsTheFileThatImportedIt checks the import's body runs
// first, so its failure is what the program reports.
func TestAFailingModuleStopsTheFileThatImportedIt(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"main.jin": "import \"u\" as u\nprint(\"never printed\")",
		"u.jin":    "let n = 0\nprint(10 / n)",
	})

	for _, run := range []struct {
		name string
		fn   func(*testing.T, string) (string, error)
	}{
		{"vm", runFileOnVM},
		{"interpreter", runFileOnInterpreter},
	} {
		t.Run(run.name, func(t *testing.T) {
			out, err := run.fn(t, filepath.Join(dir, "main.jin"))
			if err == nil {
				t.Fatal("expected the import to fail")
			}
			if want := "u.jin:2:10: division by zero"; err.Error() != want {
				t.Errorf("error = %q, want %q", err.Error(), want)
			}
			if out != "" {
				t.Errorf("output = %q, want nothing", out)
			}
		})
	}
}

// ---------------------------------------------------------------- repl

// TestTheExampleProjectAgreesAcrossEngines runs the shipped project on both
// back ends, with and without arguments, so the example is known to work on
// either.
func TestTheExampleProjectAgreesAcrossEngines(t *testing.T) {
	m, _, err := NewLoader().LoadProject(filepath.Join("examples", "greeting"))
	if err != nil {
		t.Fatal(err)
	}
	if errs := CheckProject(m); len(errs) > 0 {
		t.Fatalf("the example project should type-check, got: %s", strings.Join(errs, "; "))
	}

	cases := []struct {
		name        string
		programArgs []string
		want        string
	}{
		{name: "no arguments", want: "Hello, world!\n"},
		{name: "one argument", programArgs: []string{"Jin"}, want: "Hello, Jin!\n"},
		{name: "two arguments", programArgs: []string{"Jin", "Jo"}, want: "Hello, Jin!\nHello, Jo!\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vmOut, err := runModuleOnVM(t, m, tc.programArgs)
			if err != nil {
				t.Fatal(err)
			}
			interpOut, err := runModuleOnInterpreter(t, m, tc.programArgs)
			if err != nil {
				t.Fatal(err)
			}
			if vmOut != interpOut {
				t.Fatalf("engines disagree:\n vm:\n%s\n interp:\n%s", vmOut, interpOut)
			}
			if vmOut != tc.want {
				t.Errorf("output = %q, want %q", vmOut, tc.want)
			}
		})
	}
}

// TestAReplLineCanImportAndLaterLinesCanUseIt checks the REPL keeps an import
// alive between lines, on the bytecode back end it actually uses.
func TestAReplLineCanImportAndLaterLinesCanUseIt(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"utils.jin": "fn double(n: int) -> int {\n    return n * 2\n}\nlet name = \"utils\"",
	})

	compiler := NewCompiler(NewSymbolTable(true))
	loader := NewLoader()
	checker := NewChecker()
	checked := map[*Module]map[string]*Type{}

	var out bytes.Buffer
	vm := NewVM(&out)

	run := func(line string) {
		t.Helper()

		p := NewParser(NewLexer(line))
		program := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parse errors for %q: %v", line, errs)
		}

		m, err := loader.LoadStatements(program, dir)
		if err != nil {
			t.Fatalf("loading %q: %v", line, err)
		}
		if errs := checker.CheckModules(m, checked); len(errs) > 0 {
			t.Fatalf("type errors for %q: %s", line, strings.Join(errs, "; "))
		}
		bc, err := compiler.CompileModules(m)
		if err != nil {
			t.Fatalf("compiling %q: %v", line, err)
		}
		if _, err := vm.Run(bc); err != nil {
			t.Fatalf("running %q: %v", line, err)
		}
	}

	run(`import "utils" as u`)
	run(`print(u.double(4), u.name)`)
	run(`print(u.double(1))`)

	if got, want := out.String(), "8 utils\n2\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTheArgumentsCanBeGivenToRun checks a project's entry point can be told
// what it is being asked to do, the way a command is.
func TestProgramsCanBeReachedThroughAProject(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"jin.toml": "name = \"demo\"\nversion = \"1.2.3\"\n",
		"main.jin": "import \"greet\" as g\n" +
			"let who = args()\n" +
			"if len(who) == 0 {\n" +
			"    print(g.hello(\"world\"))\n" +
			"} else {\n" +
			"    print(g.hello(who[0]))\n" +
			"}",
		"greet.jin": "fn hello(who: str) -> str {\n    return \"Hello, {who}!\"\n}",
	})

	m, manifest, err := NewLoader().LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Label() != "demo 1.2.3" {
		t.Errorf("label = %q, want %q", manifest.Label(), "demo 1.2.3")
	}

	for _, run := range []struct {
		name string
		fn   func(*testing.T, *Module, []string) (string, error)
	}{
		{"vm", runModuleOnVM},
		{"interpreter", runModuleOnInterpreter},
	} {
		t.Run(run.name, func(t *testing.T) {
			got, err := run.fn(t, m, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != "Hello, world!\n" {
				t.Errorf("output = %q, want %q", got, "Hello, world!\n")
			}

			got, err = run.fn(t, m, []string{"Jin"})
			if err != nil {
				t.Fatal(err)
			}
			if got != "Hello, Jin!\n" {
				t.Errorf("output with an argument = %q, want %q", got, "Hello, Jin!\n")
			}
		})
	}
}

func runModuleOnVM(t *testing.T, m *Module, programArgs []string) (string, error) {
	t.Helper()

	bc, err := NewCompiler(NewSymbolTable(true)).CompileModules(m)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	vm := NewVMWithArgs(&out, programArgs)
	_, err = vm.Run(bc)
	return out.String(), err
}

func runModuleOnInterpreter(t *testing.T, m *Module, programArgs []string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	interp := NewInterpreterWithArgs(&out, programArgs)
	_, err := interp.RunModules(m, NewEnvironment())
	return out.String(), err
}

func TestImportedErrorCaughtByImporter(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"math.jin": `fn div(a, b) {
    return a / b
}`,
		"main.jin": `import "math" as m
try {
    m.div(10, 0)
} catch e {
    print("caught error:", e)
}`,
	})

	vmOut, vmErr := runFileOnVM(t, filepath.Join(dir, "main.jin"))
	if vmErr != nil {
		t.Fatalf("vm error: %v", vmErr)
	}
	interpOut, interpErr := runFileOnInterpreter(t, filepath.Join(dir, "main.jin"))
	if interpErr != nil {
		t.Fatalf("interp error: %v", interpErr)
	}

	if vmOut != interpOut {
		t.Errorf("outputs differ:\n    vm: %q\ninterp: %q", vmOut, interpOut)
	}
	if !strings.Contains(vmOut, "math.jin:2:14: division by zero") {
		t.Errorf("expected file:line:col prefix in error message, got %q", vmOut)
	}
}
