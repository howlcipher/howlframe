package observer_test

import (
	"os"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/backend/gogen"
	"github.com/howlcipher/howlframe/observer"
)

func TestGetOptimizedPluginAlwaysDisabled(t *testing.T) {
	fn, ok := observer.GetOptimizedPlugin("test_metric")
	if ok || fn != nil {
		t.Fatalf("expected GetOptimizedPlugin to return nil, false; got non-nil func, ok=%v", ok)
	}
}

func TestOptimizeGoImplementationNoExecution(t *testing.T) {
	tmpFile := "telemetry.jsonl"
	_ = os.Remove(tmpFile)
	defer os.Remove(tmpFile)

	// Call OptimizeGoImplementation; it must not create any .go or .so files
	observer.OptimizeGoImplementation("metric_foo", "func main() {}")

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".so") || strings.HasPrefix(e.Name(), "plugin_") {
			t.Fatalf("found stray plugin build artifact %s", e.Name())
		}
	}
}

func TestRecordOptimizationOpportunity(t *testing.T) {
	tmpFile := "telemetry.jsonl"
	_ = os.Remove(tmpFile)
	defer os.Remove(tmpFile)

	observer.RecordOptimizationOpportunity("fast_query", 120)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read telemetry: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "optimization_opportunity") || !strings.Contains(content, "fast_query") {
		t.Fatalf("telemetry did not contain expected event, got: %s", content)
	}
}

func TestOptimizeBlockCodegenDoesNotEmitPluginLoading(t *testing.T) {
	// (http_server "8080" (route "/test" (lambda (req) (optimize_block "query_latency" 100 (+ 1 2)))))
	optNode := &ast.Node{
		Type: "List",
		Children: []*ast.Node{
			{Type: "SYMBOL", Value: "optimize_block"},
			{Type: "STRING", Value: "query_latency"},
			{Type: "INT", Value: "100"},
			{
				Type: "List",
				Children: []*ast.Node{
					{Type: "SYMBOL", Value: "+"},
					{Type: "INT", Value: "1"},
					{Type: "INT", Value: "2"},
				},
			},
		},
	}

	lambdaNode := &ast.Node{
		Type: "List",
		Children: []*ast.Node{
			{Type: "SYMBOL", Value: "lambda"},
			{
				Type: "List",
				Children: []*ast.Node{
					{Type: "SYMBOL", Value: "req"},
				},
			},
			optNode,
		},
	}

	routeNode := &ast.Node{
		Type: "List",
		Children: []*ast.Node{
			{Type: "SYMBOL", Value: "route"},
			{Type: "STRING", Value: "/test"},
			lambdaNode,
		},
	}

	appNode := &ast.Node{
		Type: "List",
		Children: []*ast.Node{
			{Type: "SYMBOL", Value: "http_server"},
			{Type: "STRING", Value: "8080"},
			routeNode,
		},
	}

	code, _ := gogen.GenerateCode(appNode)

	forbidden := []string{
		"buildmode=plugin",
		"plugin.Open",
		"GetOptimizedPlugin",
		"OptimizeGoImplementation",
	}

	for _, token := range forbidden {
		if strings.Contains(code, token) {
			t.Fatalf("generated Go code contains forbidden dynamic plugin token %q:\n%s", token, code)
		}
	}

	if !strings.Contains(code, "RecordOptimizationOpportunity") {
		t.Fatalf("expected generated code to call RecordOptimizationOpportunity:\n%s", code)
	}
}
