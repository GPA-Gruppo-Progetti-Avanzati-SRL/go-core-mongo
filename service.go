package coremongo

import (
	"context"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.uber.org/fx"
)

// Service è il servizio Mongo: promuove i metodi della LinkedService sottostante,
// espone le operazioni CRUD generiche e possiede le aggregation caricate al boot.
type Service struct {
	*mongolks.LinkedService
	aggregations aggregations
}

// serviceParams raccoglie le dipendenze di newService. aggregationSource è
// optional perché il Module la supplisce solo quando l'app passa
// WithAggregations: se manca, il servizio parte senza aggregation.
type serviceParams struct {
	core.In
	Config       *Config
	Lifecycle    fx.Lifecycle
	Aggregations aggregationSource `optional:"true"`
}

func newService(p serviceParams) (*Service, error) {

	mls, err := mongolks.NewLinkedServiceWithConfig(*p.Config)
	if err != nil {
		return nil, err
	}

	// Prima della connessione: una cartella di aggregation malformata deve
	// fermare l'avvio dell'app, non emergere alla prima query.
	aggs, errAggr := loadAggregations(p.Aggregations.dir)
	if errAggr != nil {
		return nil, errAggr
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return mls.Connect(ctx)

		},
		OnStop: func(ctx context.Context) error {
			mls.Disconnect(ctx)
			return nil
		}})

	return &Service{LinkedService: mls, aggregations: aggs}, nil

}
