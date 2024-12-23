package mongo

import (
	"context"
	"crypto/tls"
	"go.uber.org/fx"
	"strconv"
	"time"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	mongoprom "github.com/globocom/mongo-go-prometheus"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
	"go.opentelemetry.io/otel"
)

type Service struct {
	client   *mongo.Client
	Database *mongo.Database
}

var DefaultWriteConcern = writeconcern.Majority()
var DefaultReadConcern = readconcern.Majority()

const DefaultWriteTimeout = 60 * time.Second
const DefaultAuthMechanism = "SCRAM-SHA-256"

func EvalWriteConcern(wstr string) *writeconcern.WriteConcern {

	w := DefaultWriteConcern
	if wstr != "" {
		switch wstr {
		case "majority":
			w = writeconcern.Majority()
		case "1":
			w = writeconcern.W1()
		default:
			if i, err := strconv.Atoi(wstr); err == nil {
				w = &writeconcern.WriteConcern{W: i}
			}
		}
	}

	return w
}

func NewService(config *Config, lc fx.Lifecycle) *Service {

	mongoService := &Service{}
	opts := configureMongo(config)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var err error
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

	return mongoService

}

func configureMongo(cfg *Config) *options.ClientOptions {
	opts := options.Client()

	opts.Monitor = combineMonitors(
		otelmongo.NewMonitor(otelmongo.WithTracerProvider(otel.GetTracerProvider())),
		mongoprom.NewCommandMonitor(
			mongoprom.WithInstanceName(""),
			mongoprom.WithNamespace(""),
		),
	)

	opts.ApplyURI(cfg.Server).
		SetWriteConcern(EvalWriteConcern(cfg.WriteConcern))

	// setMongoOptions
	setMongoOptions(cfg, opts)

	readConcern := DefaultReadConcern

	if cfg.ReadConcern != "" {
		readConcern = &readconcern.ReadConcern{Level: cfg.ReadConcern}
	}

	opts.SetReadConcern(readConcern)

	switch cfg.SecurityProtocol {
	case "TLS":
		log.Info().Bool("skip-verify", cfg.TLS.SkipVerify).Msg("mongo security-protocol set to TLS....")
		tlsCfg := &tls.Config{
			InsecureSkipVerify: cfg.TLS.SkipVerify,
		}
		opts.SetTLSConfig(tlsCfg)
	case "PLAIN":
		log.Info().Str("security-protocol", cfg.SecurityProtocol).Msg("mongo security-protocol set to PLAIN....nothing to do")
	default:
		log.Warn().Str("security-protocol", cfg.SecurityProtocol).Msg("mongo implicit security-protocol to PLAIN")
	}

	if cfg.Username != "" && cfg.Password != "" {
		authMechanism := DefaultAuthMechanism
		if cfg.AuthMechanism != "" {
			authMechanism = cfg.AuthMechanism
		}

		opts.Auth = &options.Credential{
			AuthSource:    cfg.AuthDB,
			Username:      cfg.Username,
			Password:      cfg.Password,
			AuthMechanism: authMechanism,
		}
	}
	return opts
}

func combineMonitors(monitors ...*event.CommandMonitor) *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			for _, monitor := range monitors {
				if monitor != nil && monitor.Started != nil {
					monitor.Started(ctx, evt)
				}
			}
		},
		Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
			for _, monitor := range monitors {
				if monitor != nil && monitor.Succeeded != nil {
					monitor.Succeeded(ctx, evt)
				}
			}
		},
		Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
			for _, monitor := range monitors {
				if monitor != nil && monitor.Failed != nil {
					monitor.Failed(ctx, evt)
				}
			}
		},
	}
}

func setMongoOptions(cfg *Config, opts *options.ClientOptions) {
	if cfg.Pool != nil {
		setConnectionPoolSettings(cfg, opts)
	}

	if cfg.WriteConcern != "" {
		opts.SetWriteConcern(EvalWriteConcern(cfg.WriteConcern))
	}

	if cfg.OperationTimeout != nil {
		opts.SetTimeout(*cfg.OperationTimeout)
	}

	if cfg.HeartbeatInterval != nil {
		opts.SetHeartbeatInterval(*cfg.HeartbeatInterval)
	}

	if cfg.ServerSelectionTimeout != nil {
		opts.SetServerSelectionTimeout(*cfg.ServerSelectionTimeout)
	}

	if cfg.RetryReads != nil {
		opts.SetRetryReads(*cfg.RetryReads)
	}

	if cfg.RetryWrites != nil {
		opts.SetRetryWrites(*cfg.RetryWrites)
	}

	if len(cfg.Compressor) > 0 {
		opts.SetCompressors(cfg.Compressor)
	}

	if cfg.ZlibLevel != nil {
		opts.SetZlibLevel(*cfg.ZlibLevel)
	}

	if cfg.ZstdLevel != nil {
		opts.SetZstdLevel(*cfg.ZstdLevel)
	}

}

func setConnectionPoolSettings(cfg *Config, opts *options.ClientOptions) {
	if cfg.Pool.MinConn != nil {
		opts.SetMinPoolSize(*cfg.Pool.MinConn)
	}
	if cfg.Pool.MaxConn != nil {
		opts.SetMaxPoolSize(*cfg.Pool.MaxConn)
	}
	if cfg.Pool.MaxConnectionIdleTime != nil {
		opts.SetMaxConnIdleTime(*cfg.Pool.MaxConnectionIdleTime)
	}
	if cfg.Pool.MaxWaitTime != nil {
		opts.SetConnectTimeout(*cfg.Pool.MaxWaitTime)
	}
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
