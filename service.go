package mongo

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/fx"
)

type Core struct {
	fx.In
	AggregationFiles AggregationDirectory `optional:"true"`
}

type Service struct {
	client     *mongo.Client
	Database   *mongo.Database
	poolMetric *poolMetric
}

func NewService(config *Config, lc fx.Lifecycle, mc Core) *Service {

	mongoService := &Service{}

	mongoService.poolMetric = &poolMetric{}
	mongoService.poolMetric.init(config.MetricConfig)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var err error
			opts := configureMongo(config, mongoService.poolMetric)
			mongoService.client, err = mongo.Connect(ctx, opts)
			mongoService.Database = mongoService.client.Database(config.Database)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to MongoDB")
				return err
			}
			err2 := mongoService.client.Ping(context.TODO(), nil)
			if err2 != nil {
				log.Fatal().Err(err).Msg("Failed to ping MongoDB")
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {

			if mongoService.client != nil {
				log.Info().Msg("Disconnetting MongoDB")
				errDis := mongoService.client.Disconnect(ctx)
				if errDis != nil {
					log.Fatal().Err(errDis).Msg("Failed disconnect MongoDB")
				}
				return errDis
			}
			return nil
		}})

	if config.Aggregations != "" {
		LoadAggregations(config.Aggregations, embed.FS(mc.AggregationFiles))
	}

	return mongoService

}

func (ms *Service) ExecTransaction(ctx context.Context, transaction func(sessCtx mongo.SessionContext) error) *core.ApplicationError {
	wc := writeconcern.Majority()
	txnOptions := options.Transaction().SetWriteConcern(wc)
	// Starts a session on the client
	session, err := ms.client.StartSession()
	if err != nil {
		return core.TechnicalErrorWithError(err)
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
	if err != nil {
		return core.TechnicalErrorWithError(err)
	}
	return nil

}

func GetObjectById[T any](ctx context.Context, ms *Service, id, collection string, obj *T) *core.ApplicationError {

	filter := bson.D{
		bson.E{Key: "_id", Value: id},
	}
	err := ms.Database.Collection(collection).FindOne(ctx, filter).Decode(&obj)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return core.NotFoundError()
		}
		return core.TechnicalErrorWithError(err)
	}
	return nil

}

func (ms *Service) CountDocuments(ctx context.Context, collection string, filter any) (int64, *core.ApplicationError) {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return 0, core.TechnicalErrorWithError(errB)
	}
	i, err := ms.Database.Collection(collection).CountDocuments(ctx, filterB)
	if err != nil {
		return 0, core.TechnicalErrorWithError(err)
	}
	return i, nil

}

func GetObjectByFilter[T any](ctx context.Context, ms *Service, collection string, filter any, obj *T) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	err := ms.Database.Collection(collection).FindOne(ctx, filterB).Decode(&obj)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return core.NotFoundError()
		}
		return core.TechnicalErrorWithError(err)
	}
	return nil

}

func (ms *Service) ExecuteAggregation(ctx context.Context, name string, params map[string]any, opts ...*options.AggregateOptions) (*mongo.Cursor, *core.ApplicationError) {
	aggregation, ok := Aggregations[name]
	if !ok {
		return nil, core.BusinessErrorWithCodeAndMessage("NOT-FOUND", fmt.Sprintf("aggregation '%s' not found", name))
	}
	mp, err := GenerateAggregation(aggregation, params)

	if err != nil {
		return nil, err
	}

	value := MongoPipelineToJson(mp)

	log.Trace().Msg(value)
	cur, errAgg := ms.Database.Collection(aggregation.Collection).Aggregate(ctx, mp, opts...)
	if errAgg != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError()
		}
		return nil, core.TechnicalErrorWithError(errAgg)
	}
	return cur, nil
}

func MongoPipelineToJson(pipeline interface{}) string {

	mappa := bson.M{"pipeline": pipeline}

	value, err := bson.MarshalExtJSON(mappa, false, false)

	if err != nil {
		return ""
	}
	return string(value)

}
