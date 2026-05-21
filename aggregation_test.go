package coremongo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestAggregationFromFiles scansiona testdata/*.yaml e per ognuno cerca il .json gemello.
// Genera la pipeline e verifica che corrisponda all'atteso.
func TestAggregationFromFiles(t *testing.T) {
	yamlFiles, err := filepath.Glob("testdata/*.yaml")
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	if len(yamlFiles) == 0 {
		t.Fatal("nessun file yaml trovato in testdata/")
	}

	for _, yamlPath := range yamlFiles {
		name := strings.TrimSuffix(filepath.Base(yamlPath), ".yaml")
		jsonPath := filepath.Join("testdata", name+".json")

		t.Run(name, func(t *testing.T) {
			rawYAML, err := os.ReadFile(yamlPath)
			if err != nil {
				t.Fatalf("lettura yaml: %v", err)
			}

			rawJSON, err := os.ReadFile(jsonPath)
			if err != nil {
				t.Fatalf("lettura json atteso (%s): %v", jsonPath, err)
			}

			a := &Aggregation{}
			if err := yaml.Unmarshal(rawYAML, a); err != nil {
				t.Fatalf("unmarshal yaml: %v", err)
			}

			pipeline, appErr := GenerateAggregation(a, map[string]any{})
			if appErr != nil {
				t.Fatalf("generate aggregation: code=%s msg=%s", appErr.Code, appErr.Message)
			}

			got := PipelineToJson(pipeline)
			t.Logf("pipeline generata:\n%s", got)

			var gotObj, wantObj interface{}
			if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
				t.Fatalf("unmarshal pipeline generata: %v\njson: %s", err, got)
			}
			if err := json.Unmarshal(rawJSON, &wantObj); err != nil {
				t.Fatalf("unmarshal json atteso: %v", err)
			}

			if !reflect.DeepEqual(gotObj, wantObj) {
				gotPretty, _ := json.MarshalIndent(gotObj, "", "  ")
				wantPretty, _ := json.MarshalIndent(wantObj, "", "  ")
				t.Errorf("pipeline non corrisponde\n--- got ---\n%s\n--- want ---\n%s", gotPretty, wantPretty)
			}
		})
	}
}
