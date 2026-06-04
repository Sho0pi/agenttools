package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sho0pi/agenttools/tool"
)

// newWS returns an OSFileSystem rooted at a fresh temp dir seeded with files.
func newWS(t *testing.T) *OSFileSystem {
	t.Helper()
	root := t.TempDir()
	seed := map[string]string{
		"readme.md":          "# Title\nline two\nline three\n",
		"src/main.go":        "package main\n\nfunc main() { println(\"hi\") }\n",
		"src/util.go":        "package main\n\n// TODO: refactor\nfunc helper() {}\n",
		"docs/guide.txt":     "alpha\nbeta\ngamma\n",
		".hidden/secret.txt": "shh\n",
	}
	for rel, content := range seed {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ws, err := NewOSFileSystem(Config{Root: root})
	if err != nil {
		t.Fatalf("NewOSFileSystem: %v", err)
	}
	return ws
}

func call(t *testing.T, tr tool.Tool, args any) (tool.Result, error) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return tr.Execute(context.Background(), raw)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestConstructors_NilFS(t *testing.T) {
	ctors := map[string]func(FileSystem) (tool.Tool, error){
		"read": NewReadTool, "write": NewWriteTool, "edit": NewEditTool,
		"list": NewListTool, "grep": NewGrepTool,
	}
	for name, ctor := range ctors {
		if _, err := ctor(nil); err == nil {
			t.Errorf("%s: expected error for nil FileSystem", name)
		}
	}
}

func TestOSFileSystem_Containment(t *testing.T) {
	ws := newWS(t)
	ctx := context.Background()
	bad := []string{
		"../escape.txt",
		"src/../../escape.txt",
		"/etc/passwd",
		"a\x00b",
	}
	for _, p := range bad {
		if _, err := ws.ReadFile(ctx, p); err == nil {
			t.Errorf("ReadFile(%q) = nil error, want containment rejection", p)
		}
	}
}

func TestOSFileSystem_SymlinkEscape(t *testing.T) {
	ws := newWS(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws.Root(), "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if _, err := ws.ReadFile(context.Background(), "link.txt"); err == nil {
		t.Fatal("reading a symlink that escapes the workspace should fail")
	}
}

func TestOSFileSystem_SizeCap(t *testing.T) {
	root := t.TempDir()
	big := filepath.Join(root, "big.bin")
	if err := os.WriteFile(big, make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := NewOSFileSystem(Config{Root: root, MaxReadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.ReadFile(context.Background(), "big.bin"); err == nil {
		t.Fatal("reading a file over the size cap should fail")
	}
}

func TestReadTool(t *testing.T) {
	tr := must(NewReadTool(newWS(t)))

	res, err := call(t, tr, ReadArgs{Path: "readme.md", IncludeLineNumbers: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "1: # Title") || !strings.Contains(res.Content, "3: line three") {
		t.Fatalf("numbered read wrong:\n%s", res.Content)
	}
	if res.Data["total_lines"] != 3 {
		t.Fatalf("total_lines = %v, want 3", res.Data["total_lines"])
	}

	res, err = call(t, tr, ReadArgs{Path: "readme.md", StartLine: 2, EndLine: 2})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(res.Content) != "line two" {
		t.Fatalf("range read = %q, want 'line two'", res.Content)
	}

	if _, err := call(t, tr, ReadArgs{Path: "nope.txt"}); err == nil {
		t.Fatal("reading a missing file should error")
	}
}

func TestWriteTool(t *testing.T) {
	ws := newWS(t)
	tr := must(NewWriteTool(ws))
	ctx := context.Background()

	// create new
	if _, err := call(t, tr, WriteArgs{Path: "new/file.txt", Content: "hello", CreateDirs: true}); err != nil {
		t.Fatal(err)
	}
	got, _ := ws.ReadFile(ctx, "new/file.txt")
	if string(got) != "hello" {
		t.Fatalf("content = %q", got)
	}

	// create over existing fails
	if _, err := call(t, tr, WriteArgs{Path: "new/file.txt", Content: "x"}); err == nil {
		t.Fatal("create over existing should fail")
	}

	// overwrite with backup
	if _, err := call(t, tr, WriteArgs{Path: "new/file.txt", Content: "world", Mode: "overwrite", Backup: true}); err != nil {
		t.Fatal(err)
	}
	got, _ = ws.ReadFile(ctx, "new/file.txt")
	if string(got) != "world" {
		t.Fatalf("after overwrite = %q", got)
	}
	if bak, _ := ws.ReadFile(ctx, "new/file.txt.bak"); string(bak) != "hello" {
		t.Fatalf("backup = %q, want 'hello'", bak)
	}

	// append
	if _, err := call(t, tr, WriteArgs{Path: "new/file.txt", Content: "!", Mode: "append"}); err != nil {
		t.Fatal(err)
	}
	got, _ = ws.ReadFile(ctx, "new/file.txt")
	if string(got) != "world!" {
		t.Fatalf("after append = %q", got)
	}

	// invalid mode
	if _, err := call(t, tr, WriteArgs{Path: "x.txt", Content: "y", Mode: "bogus"}); err == nil {
		t.Fatal("invalid mode should error")
	}
}

func TestEditTool(t *testing.T) {
	ws := newWS(t)
	tr := must(NewEditTool(ws))
	ctx := context.Background()

	res, err := call(t, tr, EditArgs{Path: "docs/guide.txt", OldString: "beta", NewString: "BETA"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["replacements"] != 1 {
		t.Fatalf("replacements = %v", res.Data["replacements"])
	}
	got, _ := ws.ReadFile(ctx, "docs/guide.txt")
	if !strings.Contains(string(got), "BETA") {
		t.Fatalf("edit not applied: %q", got)
	}

	// not found
	if _, err := call(t, tr, EditArgs{Path: "docs/guide.txt", OldString: "zzz", NewString: "y"}); err == nil {
		t.Fatal("missing old_string should error")
	}
	// identical
	if _, err := call(t, tr, EditArgs{Path: "docs/guide.txt", OldString: "a", NewString: "a"}); err == nil {
		t.Fatal("identical old/new should error")
	}
	// dry run does not write
	before, _ := ws.ReadFile(ctx, "docs/guide.txt")
	if _, err := call(t, tr, EditArgs{Path: "docs/guide.txt", OldString: "gamma", NewString: "GAMMA", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	after, _ := ws.ReadFile(ctx, "docs/guide.txt")
	if string(before) != string(after) {
		t.Fatal("dry_run must not modify the file")
	}
}

func TestListTool(t *testing.T) {
	tr := must(NewListTool(newWS(t)))

	res, err := call(t, tr, ListArgs{Path: "."})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "[DIR]  src/") || !strings.Contains(res.Content, "readme.md") {
		t.Fatalf("list wrong:\n%s", res.Content)
	}
	if strings.Contains(res.Content, ".hidden") {
		t.Fatal("hidden dir should be excluded by default")
	}

	res, err = call(t, tr, ListArgs{Path: ".", Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "src/main.go") {
		t.Fatalf("recursive list missing nested file:\n%s", res.Content)
	}

	res, err = call(t, tr, ListArgs{Path: ".", IncludeHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, ".hidden") {
		t.Fatal("include_hidden should show dotfiles")
	}
}

func TestGrepTool(t *testing.T) {
	tr := must(NewGrepTool(newWS(t)))

	res, err := call(t, tr, GrepArgs{Pattern: "TODO"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "src/util.go:3:") {
		t.Fatalf("grep wrong:\n%s", res.Content)
	}

	// glob restriction: search only .txt → no Go TODO
	res, err = call(t, tr, GrepArgs{Pattern: "TODO", Glob: "**/*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "No matches") {
		t.Fatalf("glob restriction failed:\n%s", res.Content)
	}

	// regex
	res, err = call(t, tr, GrepArgs{Pattern: "func \\w+\\(", Regex: true, Glob: "**/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["matches"].(int) < 2 {
		t.Fatalf("regex matches = %v, want >=2", res.Data["matches"])
	}

	// case sensitivity
	res, _ = call(t, tr, GrepArgs{Pattern: "todo", CaseSensitive: true})
	if !strings.Contains(res.Content, "No matches") {
		t.Fatalf("case-sensitive 'todo' should not match 'TODO':\n%s", res.Content)
	}
	res, _ = call(t, tr, GrepArgs{Pattern: "todo"})
	if strings.Contains(res.Content, "No matches") {
		t.Fatal("case-insensitive 'todo' should match 'TODO'")
	}

	// context lines
	res, err = call(t, tr, GrepArgs{Pattern: "beta", Glob: "**/*.txt", ContextLines: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "alpha") || !strings.Contains(res.Content, "gamma") {
		t.Fatalf("context lines missing:\n%s", res.Content)
	}
}
