package compress

import (
	"fmt"
	"strings"
	"testing"
)

// --- JSON array ---

func TestJSONArray_collapsed(t *testing.T) {
	items := make([]string, 10)
	for i := range items {
		items[i] = fmt.Sprintf(`{"id":%d,"name":"user-%d"}`, i, i)
	}
	content := "[" + strings.Join(items, ",") + "]"

	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("JSON array with >3 items must be collapsed")
	}
	if !strings.Contains(out, "…and 8 more items") {
		t.Fatalf("expected ellipsis summary, got: %s", out)
	}
	if len(out) >= len(content) {
		t.Fatal("collapsed output must be shorter")
	}
}

func TestJSONArray_tooFew(t *testing.T) {
	content := `[{"id":1},{"id":2}]`
	_, ok := CollapseStructure(content, nil)
	if ok {
		t.Fatal("array with <=3 items must not be collapsed")
	}
}

func TestJSONArray_invalid(t *testing.T) {
	_, ok := CollapseStructure(`[not valid json`, nil)
	if ok {
		t.Fatal("invalid JSON must not be collapsed")
	}
}

func TestJSONArray_object(t *testing.T) {
	_, ok := CollapseStructure(`{"key":"value"}`, nil)
	if ok {
		t.Fatal("JSON object (not array) must not be collapsed")
	}
}

func TestJSONObject_collapsed(t *testing.T) {
	content := `{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8,"i":9}`
	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("large JSON object must be collapsed")
	}
	if !strings.Contains(out, "more keys") {
		t.Fatalf("expected key summary, got: %s", out)
	}
}

func TestNDJSON_collapsed(t *testing.T) {
	content := strings.Join([]string{
		`{"id":1,"name":"a"}`,
		`{"id":2,"name":"b"}`,
		`{"id":3,"name":"c"}`,
		`{"id":4,"name":"d"}`,
	}, "\n")
	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("NDJSON must be collapsed")
	}
	if !strings.Contains(out, "JSON records") {
		t.Fatalf("expected JSON records marker, got: %s", out)
	}
}

func TestCSV_collapsed(t *testing.T) {
	content := "name,status,age\napi,running,1d\nworker,running,2d\ndb,running,3d\ncache,running,4d"
	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("CSV must be collapsed")
	}
	if !strings.Contains(out, "more rows") {
		t.Fatalf("expected more rows marker, got: %s", out)
	}
}

func TestYAML_collapsed(t *testing.T) {
	var lines []string
	for i := range 14 {
		lines = append(lines, "service_"+itoa(i)+": enabled")
	}
	out, ok := CollapseStructure(strings.Join(lines, "\n"), nil)
	if !ok {
		t.Fatal("YAML-like output must be collapsed")
	}
	if !strings.Contains(out, "YAML lines") {
		t.Fatalf("expected YAML summary, got: %s", out)
	}
}

func TestTOML_collapsed(t *testing.T) {
	content := `[database]
host = "localhost"
port = 5432
[cache]
enabled = true
size = 20
[queue]
enabled = true
workers = 4
timeout = 30`
	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("TOML-like output must be collapsed")
	}
	if !strings.Contains(out, "sections") {
		t.Fatalf("expected TOML summary, got: %s", out)
	}
}

func TestUnifiedDiff_collapsed(t *testing.T) {
	var lines []string
	lines = append(lines, "diff --git a/app.go b/app.go", "--- a/app.go", "+++ b/app.go", "@@ -1,20 +1,20 @@")
	for i := range 12 {
		lines = append(lines, "-old line "+itoa(i), "+new line "+itoa(i))
	}
	out, ok := CollapseStructure(strings.Join(lines, "\n"), nil)
	if !ok {
		t.Fatal("large unified diff must be collapsed")
	}
	if !strings.Contains(out, "additions") {
		t.Fatalf("expected diff summary, got: %s", out)
	}
}

func TestDiagnostics_collapsed(t *testing.T) {
	var lines []string
	for i := range 9 {
		lines = append(lines, "src/app.go:"+itoa(10+i)+":1: cannot use value")
	}
	out, ok := CollapseStructure(strings.Join(lines, "\n"), nil)
	if !ok {
		t.Fatal("diagnostics must be collapsed")
	}
	if !strings.Contains(out, "more diagnostics") {
		t.Fatalf("expected diagnostics summary, got: %s", out)
	}
}

func TestMarkup_collapsed(t *testing.T) {
	content := "<html><body>" + strings.Repeat("<div><span>item</span></div>", 12) + "</body></html>"
	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("markup must be collapsed")
	}
	if !strings.Contains(out, "markup") {
		t.Fatalf("expected markup summary, got: %s", out)
	}
}

func TestCoverage_collapsed(t *testing.T) {
	content := `mode: set
pkg/a.go:10.1,12.2 1 0
pkg/a.go:13.1,14.2 1 0
pkg/a.go:15.1,16.2 1 0
pkg/a.go:17.1,18.2 1 0
pkg/a.go:19.1,20.2 1 0
pkg/a.go:21.1,22.2 1 0
coverage: 42.0% of statements
total: 42.0%`
	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("coverage output must be collapsed")
	}
	if !strings.Contains(out, "total") {
		t.Fatalf("expected total coverage line, got: %s", out)
	}
}

func TestPackageLog_collapsed(t *testing.T) {
	var lines []string
	lines = append(lines, "npm install")
	for i := range 12 {
		lines = append(lines, "npm WARN deprecated package-"+itoa(i))
	}
	lines = append(lines, "added 42 packages")
	out, ok := CollapseStructure(strings.Join(lines, "\n"), nil)
	if !ok {
		t.Fatal("package log must be collapsed")
	}
	if !strings.Contains(out, "WARN") {
		t.Fatalf("expected warning lines, got: %s", out)
	}
	if !strings.Contains(out, "package-log: npm") {
		t.Fatalf("expected npm summary, got: %s", out)
	}
}

func TestPackageLog_pipAndCargo(t *testing.T) {
	pip := "pip install demo\n" + strings.Repeat("Successfully installed package\n", 12)
	if out, ok := CollapseStructure(pip, nil); !ok || !strings.Contains(out, "package-log: pip") {
		t.Fatalf("expected pip package log collapse, ok=%t out=%s", ok, out)
	}

	cargo := "cargo build\n" + strings.Repeat("Compiling crate v1.0.0\n", 12)
	if out, ok := CollapseStructure(cargo, nil); !ok || !strings.Contains(out, "package-log: cargo") {
		t.Fatalf("expected cargo package log collapse, ok=%t out=%s", ok, out)
	}
}

// --- Stack trace ---

func TestStackTrace_goRuntime(t *testing.T) {
	content := `goroutine 1 [running]:
runtime/debug.Stack()
	/usr/local/go/src/runtime/debug/stack.go:24 +0x5b
main.doWork(...)
	/home/user/app/main.go:42 +0x1a3
main.handler(...)
	/home/user/app/main.go:88 +0x88
main.middleware(...)
	/home/user/app/main.go:34 +0x62
net/http.(*ServeMux).ServeHTTP(...)
	/usr/local/go/src/net/http/server.go:124 +0x77
net/http.(*Server).Serve(...)
	/usr/local/go/src/net/http/server.go:93 +0x4f
main.main()
	/home/user/app/main.go:18 +0x2c`

	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("Go stack trace must be collapsed")
	}
	if len(out) >= len(content) {
		t.Fatal("collapsed stack trace must be shorter")
	}
	if !strings.Contains(out, "frames omitted") {
		t.Fatalf("expected frames-omitted marker, got: %s", out)
	}
}

func TestStackTrace_noHeader(t *testing.T) {
	// Indented lines but no panic/goroutine header — must not collapse
	content := strings.Repeat("\t/some/path/file.go:42 +0x1a3\n", 10)
	_, ok := CollapseStructure(content, nil)
	if ok {
		t.Fatal("must not collapse without panic/goroutine header")
	}
}

func TestStackTrace_goSource(t *testing.T) {
	// Go source code has indented lines — must not be mistaken for a stack trace
	content := `package main

import "fmt"

func main() {
	fmt.Println("hello")
	doWork()
	cleanup()
}

func doWork() {
	for i := 0; i < 10; i++ {
		process(i)
	}
}
`
	_, ok := CollapseStructure(content, nil)
	if ok {
		t.Fatal("Go source code must not be collapsed as stack trace")
	}
}

// --- File tree ---

func TestFileTree_collapsed(t *testing.T) {
	var lines []string
	lines = append(lines, ".")
	lines = append(lines, "├── cmd/")
	for i := range 15 {
		lines = append(lines, fmt.Sprintf("│   ├── file%02d.go", i))
	}
	lines = append(lines, "└── README.md")
	content := strings.Join(lines, "\n")

	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("file tree with >10 children must be collapsed")
	}
	if !strings.Contains(out, "more items") {
		t.Fatalf("expected 'more items' marker, got: %s", out)
	}
	if len(out) >= len(content) {
		t.Fatal("collapsed tree must be shorter")
	}
}

func TestFileTree_tooSmall(t *testing.T) {
	content := ".\n├── a\n└── b\n"
	_, ok := CollapseStructure(content, nil)
	if ok {
		t.Fatal("small tree must not be collapsed")
	}
}

// --- Test output ---

func TestTestOutput_collapsesPassing(t *testing.T) {
	var lines []string
	for i := range 20 {
		lines = append(lines, fmt.Sprintf("--- PASS: TestFeature%02d (0.00s)", i))
	}
	lines = append(lines, "--- FAIL: TestAuth (0.05s)")
	lines = append(lines, "    auth_test.go:42: want 200 got 401")
	lines = append(lines, "FAIL\tgithub.com/example/app\t0.12s")
	content := strings.Join(lines, "\n")

	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("test output with many passes must be collapsed")
	}
	if strings.Contains(out, "PASS: TestFeature") {
		t.Fatal("passing tests must be removed from output")
	}
	if !strings.Contains(out, "TestAuth") {
		t.Fatal("failing test must be retained")
	}
	if !strings.Contains(out, "auth_test.go:42") {
		t.Fatal("failure detail must be retained")
	}
}

func TestTestOutput_allPass(t *testing.T) {
	var lines []string
	for i := range 10 {
		lines = append(lines, fmt.Sprintf("--- PASS: TestFoo%d (0.00s)", i))
	}
	lines = append(lines, "ok  \tgithub.com/example/app\t0.05s")
	content := strings.Join(lines, "\n")

	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("all-pass output must be collapsed")
	}
	if !strings.Contains(out, "passed") {
		t.Fatalf("expected pass summary, got: %s", out)
	}
}

func TestTestOutput_tooFewPasses(t *testing.T) {
	content := "--- PASS: TestA (0.00s)\n--- PASS: TestB (0.00s)\nok\t.\t0.01s"
	_, ok := CollapseStructure(content, nil)
	if ok {
		t.Fatal("fewer than 5 passing tests must not be collapsed")
	}
}

// --- Log stream ---

func TestLogStream_deduplicates(t *testing.T) {
	var lines []string
	for range 20 {
		lines = append(lines, "2024-01-15 10:00:01 INFO health check ok")
	}
	lines = append(lines, "2024-01-15 10:00:22 INFO deploy complete")
	content := strings.Join(lines, "\n")

	out, ok := CollapseStructure(content, nil)
	if !ok {
		t.Fatal("log stream with repeated lines must be collapsed")
	}
	if !strings.Contains(out, "×") {
		t.Fatalf("expected ×N dedup marker, got: %s", out)
	}
	if len(out) >= len(content) {
		t.Fatal("deduped log must be shorter")
	}
}

func TestLogStream_tooSmall(t *testing.T) {
	content := "2024-01-15 10:00:00 INFO start\n2024-01-15 10:00:01 INFO stop\n"
	_, ok := CollapseStructure(content, nil)
	if ok {
		t.Fatal("log stream with <10 lines must not be collapsed")
	}
}

func TestLogStream_noTimestamps(t *testing.T) {
	content := strings.Repeat("INFO health check ok\n", 20)
	_, ok := CollapseStructure(content, nil)
	if ok {
		t.Fatal("content without timestamps must not be treated as log stream")
	}
}

// --- Tabular ---

func TestTabular_dropsIrrelevantColumns(t *testing.T) {
	content := `NAME              STATUS    AGE    RESTARTS   IMAGE
frontend          Running   12d    0          nginx:latest
backend           Running   12d    2          myapp:v1
worker            Running   3d     0          myapp:v1
database          Running   30d    0          postgres:15`

	args := map[string]any{"command": "kubectl get pods --show STATUS NAME"}
	out, ok := CollapseStructure(content, args)
	if !ok {
		t.Fatal("tabular output with referenced columns must be collapsed")
	}
	if strings.Contains(out, "RESTARTS") {
		t.Fatal("unreferenced column must be dropped")
	}
}

func TestTabular_noArgSignal(t *testing.T) {
	content := `NAME    STATUS    AGE    RESTARTS
a       Running   1d     0
b       Running   2d     0
c       Running   3d     0
d       Running   4d     0`

	// No column names in args — should not collapse
	args := map[string]any{"command": "kubectl get pods"}
	_, ok := CollapseStructure(content, args)
	if ok {
		t.Fatal("tabular without column signal in args must not be collapsed")
	}
}
