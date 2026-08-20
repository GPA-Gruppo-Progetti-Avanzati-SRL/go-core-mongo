package coremongo

import (
	"context"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.uber.org/fx"
)

// Service è il servizio Mongo: promuove i metodi della LinkedService sottostante,
// espone le operazioni CRUD generiche e possiede le aggregation caricate al boot.
type Service struct {
	*mongolks.LinkedService
	aggregations aggregations
}

func newService(config *Config, src aggregationSource, lc fx.Lifecycle) (*Service, error) {

	mls, err := mongolks.NewLinkedServiceWithConfig(*config)
	if err != nil {
		return nil, err
	}

	// Prima della connessione: una cartella di aggregation malformata deve
	// fermare l'avvio dell'app, non emergere alla prima query.
	aggs, errAggr := loadAggregations(src.dir)
	if errAggr != nil {
		return nil, errAggr
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return mls.Connect(ctx)

		},
		OnStop: func(ctx context.Context) error {
			mls.Disconnect(ctx)
			return nil
		}})

	return &Service{LinkedService: mls, aggregations: aggs}, nil

}
