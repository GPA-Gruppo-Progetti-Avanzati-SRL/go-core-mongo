package coremongo

import (
	"context"
	"embed"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

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

func ExecTransaction(ctx context.Context, ms *mongolks.LinkedService, transaction func(sessCtx mongo.SessionContext) error) error {
	wc := writeconcern.Majority()
	txnOptions := options.Transaction().SetWriteConcern(wc)
	// Starts a session on the client
	session, err := ms.Db().Client().StartSession()
	if err != nil {
		return err
	}

	// Defers ending the session after the transaction is committed or ended
	defer session.EndSession(ctx)

	// Esecuzione della transazione
	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		// Inizia la transazione
		if errSt := session.StartTransaction(txnOptions); errSt != nil {
			return errSt
		}

		// Esegue la transazione con il callback
		if errT := transaction(sessCtx); errT != nil {
			session.AbortTransaction(sessCtx) // Rollback
			return errT
		}

		// Commit della transazione
		return session.CommitTransaction(sessCtx)
	})
	return err
}
