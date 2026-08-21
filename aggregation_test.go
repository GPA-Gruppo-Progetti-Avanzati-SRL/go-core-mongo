package coremongo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

// TestAggregationFromFiles carica il registry da testdata/ con loadAggregations
// (quindi esercita anche il filtro .yaml, che ignora i .json gemelli) e per ogni
// pipeline confronta il generato con il golden <name>.json.
func TestAggregationFromFiles(t *testing.T) {
	reg, err := loadAggregations(os.DirFS("testdata"))
	if err != nil {
		t.Fatalf("load aggregations: %v", err)
	}
	if len(reg) == 0 {
		t.Fatal("nessuna aggregation caricata da testdata/")
	}

	for name, a := range reg {
		// I file di testdata usano basename == campo name.
		jsonPath := filepath.Join("testdata", name+".json")

		t.Run(name, func(t *testing.T) {
			rawJSON, errRead := os.ReadFile(jsonPath)
			if errRead != nil {
				t.Fatalf("lettura json atteso (%s): %v", jsonPath, errRead)
			}

			pipeline, appErr := reg.pipeline(a, map[string]any{})
			if appErr != nil {
				t.Fatalf("generate aggregation: code=%s msg=%s", appErr.Code, appErr.Message)
			}

			got := PipelineToJson(pipeline)
			t.Logf("pipeline generata:\n%s", got)

			var gotObj, wantObj any
			if errUm := json.Unmarshal([]byte(got), &gotObj); errUm != nil {
				t.Fatalf("unmarshal pipeline generata: %v\njson: %s", errUm, got)
			}
			if errUm := json.Unmarshal(rawJSON, &wantObj); errUm != nil {
				t.Fatalf("unmarshal json atteso: %v", errUm)
			}

			if !reflect.DeepEqual(gotObj, wantObj) {
				gotPretty, _ := json.MarshalIndent(gotObj, "", "  ")
				wantPretty, _ := json.MarshalIndent(wantObj, "", "  ")
				t.Errorf("pipeline non corrisponde\n--- got ---\n%s\n--- want ---\n%s", gotPretty, wantPretty)
			}
		})
	}
}

func yamlFile(body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(body)}
}

// TestLoadAggregationsNil: senza WithAggregations il servizio parte senza pipeline.
func TestLoadAggregationsNil(t *testing.T) {
	reg, err := loadAggregations(nil)
	if err != nil {
		t.Fatalf("dir nil non deve essere un errore: %v", err)
	}
	if reg != nil {
		t.Errorf("registry attesa nil, ottenuta %v", reg)
	}
}

// TestLoadAggregationsRecursive: la FS si passa così com'è, il nome della cartella
// non serve e le sottocartelle vengono percorse.
func TestLoadAggregationsRecursive(t *testing.T) {
	reg, err := loadAggregations(fstest.MapFS{
		"aggregations/uno.yaml":       yamlFile("name: uno\ncollection: c1\n"),
		"aggregations/nested/due.yml": yamlFile("name: due\ncollection: c2\n"),
		"aggregations/readme.md":      yamlFile("non una pipeline"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(reg) != 2 {
		t.Fatalf("attese 2 aggregation, ottenute %d: %v", len(reg), reg)
	}
	if reg["uno"].Collection != "c1" || reg["due"].Collection != "c2" {
		t.Errorf("registry inattesa: %+v", reg)
	}
}

// TestLoadAggregationsFailFast: ogni anomalia ferma l'avvio, non emerge a runtime.
func TestLoadAggregationsFailFast(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"nessun yaml": {
			"aggregations/listini.json": yamlFile("[]"),
		},
		"yaml non parsabile": {
			"aggregations/rotta.yaml": yamlFile("name: [rotta\n"),
		},
		"name mancante": {
			"aggregations/senzanome.yaml": yamlFile("collection: c1\n"),
		},
		"nome duplicato": {
			"aggregations/a.yaml": yamlFile("name: stesso\ncollection: c1\n"),
			"aggregations/b.yaml": yamlFile("name: stesso\ncollection: c2\n"),
		},
	}

	for name, dir := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadAggregations(dir); err == nil {
				t.Error("atteso errore, ottenuto nil")
			} else {
				t.Logf("errore atteso: %v", err)
			}
		})
	}
}
