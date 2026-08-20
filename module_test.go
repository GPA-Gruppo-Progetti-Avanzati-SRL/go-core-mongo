package coremongo

import (
	"testing"
	"testing/fstest"

	"go.uber.org/fx"
)

// TestModuleGraph valida la forma del wiring che Module registra, senza avviare
// l'app (nessun Start ⇒ nessun Connect ⇒ nessun Mongo necessario): il costruttore
// gira e le dipendenze si risolvono.
func TestModuleGraph(t *testing.T) {
	cfg := &Config{
		Name:   "default",
		Host:   "mongodb://localhost:27017",
		DbName: "test",
	}

	app := fx.New(
		fx.Supply(cfg, aggregationSource{}),
		fx.Provide(newService),
		fx.Invoke(func(s *Service) {
			// La LinkedService è embeddata per puntatore: è ciò che garantisce che il
			// Service veda la connessione aperta da OnStart. Con l'embedding per valore
			// la copia costruita qui resterebbe disconnessa.
			if s.LinkedService == nil {
				t.Error("Service.LinkedService nil")
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
	cfg := &Config{Name: "default", Host: "mongodb://localhost:27017", DbName: "test"}

	app := fx.New(
		fx.Supply(cfg, aggregationSource{dir: fstest.MapFS{}}),
		fx.Provide(newService),
		fx.Invoke(func(*Service) {}),
		fx.NopLogger,
	)

	if app.Err() == nil {
		t.Fatal("atteso errore dal costruttore per FS senza aggregation")
	}
	t.Logf("errore atteso: %v", app.Err())
}
