package coremongo

import (
	"testing"
	"testing/fstest"

	"go.uber.org/fx"
)

func testConfig() *Config {
	return &Config{
		Name:   "default",
		Host:   "mongodb://localhost:27017",
		DbName: "test",
	}
}

// TestModuleGraph valida la forma del wiring che Module registra senza WithAggregations,
// e senza avviare l'app (nessun Start ⇒ nessun Connect ⇒ nessun Mongo necessario):
// l'aggregationSource NON è nel grafo, quindi il costruttore deve risolverla come
// dipendenza optional.
func TestModuleGraph(t *testing.T) {
	app := fx.New(
		fx.Supply(testConfig()),
		fx.Provide(newService),
		fx.Invoke(func(s *Service) {
			// La LinkedService è embeddata per puntatore: è ciò che garantisce che il
			// Service veda la connessione aperta da OnStart. Con l'embedding per valore
			// la copia costruita qui resterebbe disconnessa.
			if s.LinkedService == nil {
				t.Error("Service.LinkedService nil")
			}
			if s.aggregations != nil {
				t.Errorf("senza WithAggregations il registry deve essere nil, trovato %v", s.aggregations)
			}
		}),
		fx.NopLogger,
	)

	if err := app.Err(); err != nil {
		t.Fatalf("grafo fx non valido: %v", err)
	}
}

// TestModuleGraphWithAggregations: con WithAggregations il Module supplisce
// l'aggregationSource e il registry del Service è popolato.
func TestModuleGraphWithAggregations(t *testing.T) {
	dir := fstest.MapFS{
		"aggregations/example.yaml": yamlFile("name: example\ncollection: example\n"),
	}

	app := fx.New(
		fx.Supply(testConfig(), aggregationSource{dir: dir}),
		fx.Provide(newService),
		fx.Invoke(func(s *Service) {
			if len(s.aggregations) != 1 || s.aggregations["example"] == nil {
				t.Errorf("registry inattesa: %v", s.aggregations)
			}
		}),
		fx.NopLogger,
	)

	if err := app.Err(); err != nil {
		t.Fatalf("grafo fx non valido: %v", err)
	}
}

// TestModuleGraphFailFastAggregations: una FS senza pipeline valide impedisce
// l'avvio dell'app invece di far partire il servizio con aggregation mancanti.
func TestModuleGraphFailFastAggregations(t *testing.T) {
	app := fx.New(
		fx.Supply(testConfig(), aggregationSource{dir: fstest.MapFS{}}),
		fx.Provide(newService),
		fx.Invoke(func(*Service) {}),
		fx.NopLogger,
	)

	if app.Err() == nil {
		t.Fatal("atteso errore dal costruttore per FS senza aggregation")
	}
	t.Logf("errore atteso: %v", app.Err())
}
