package coremongo

import (
	"context"
	"embed"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.uber.org/fx"
)

type Core struct {
	core.In
	AggregationFiles AggregationDirectory `optional:"true"`
	AggregationPath  *AggregationsPath    `optional:"true"`
}
type AggregationsPath string

func NewService(config *mongolks.Config, lc fx.Lifecycle, mc Core) (*mongolks.LinkedService, error) {

	mls, err := mongolks.NewLinkedServiceWithConfig(*config)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return mls.Connect(ctx)

		},
		OnStop: func(ctx context.Context) error {
			mls.Disconnect(ctx)
			return nil
		}})

	if mc.AggregationPath != nil {
		LoadAggregations(*mc.AggregationPath, embed.FS(mc.AggregationFiles))
	}

	return mls, nil

}
