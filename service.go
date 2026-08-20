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

type Service struct {
	mongolks.LinkedService
}

type AggregationsPath string

func NewService(config *mongolks.Config, lc fx.Lifecycle, mc Core) *Service {

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

	return &Service{*mls}

}
