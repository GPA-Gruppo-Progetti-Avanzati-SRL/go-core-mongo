package coremongo

import (
	"context"
	"embed"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.uber.org/fx"
)

type Core struct {
	fx.In
	AggregationFiles AggregationDirectory `optional:"true"`
}
type AggregationsPath string

func NewService(config *mongolks.Config, lc fx.Lifecycle, mc Core, aggregationPath *AggregationsPath) *mongolks.LinkedService {

	mls, _ := mongolks.NewLinkedServiceWithConfig(*config)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return mls.Connect(ctx)

		},
		OnStop: func(ctx context.Context) error {
			mls.Disconnect(ctx)
			return nil
		}})

	if aggregationPath != nil {
		LoadAggregations(*aggregationPath, embed.FS(mc.AggregationFiles))
	}

	return mls

}
