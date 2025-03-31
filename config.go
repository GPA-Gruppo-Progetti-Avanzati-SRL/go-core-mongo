package mongo

import (
	"context"
	"crypto/tls"
	mongoprom "github.com/globocom/mongo-go-prometheus"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"strconv"
	"time"
)

type Config struct {
	Server                 string         `yaml:"server" mapstructure:"server" json:"server"`
	Database               string         `yaml:"database" mapstructure:"database" json:"database"`
	Username               string         `yaml:"username" mapstructure:"username" json:"username"`
	Password               string         `yaml:"password" mapstructure:"password" json:"password"`
	AuthMechanism          string         `mapstructure:"authMechanism" json:"authMechanism" yaml:"authMechanism"`
	AuthDB                 string         `yaml:"authdb" mapstructure:"authdb" json:"authdb"`
	Pool                   *Pool          `yaml:"pool" mapstructure:"pool" json:"pool" yaml:"pool"`
	WriteConcern           string         `mapstructure:"write-concern" json:"write-concern" yaml:"write-concern"`
	ReadConcern            string         `mapstructure:"read-concern" json:"read-concern" yaml:"read-concern"`
	OperationTimeout       *time.Duration `mapstructure:"operation-timeout" json:"operation-timeout" yaml:"operation-timeout"`
	SecurityProtocol       string         `mapstructure:"security-protocol" json:"security-protocol" yaml:"security-protocol"`
	TLS                    TLSConfig      `json:"tls" mapstructure:"tls" yaml:"tls"`
	HeartbeatInterval      *time.Duration `mapstructure:"heartbeat-interval" json:"heartbeat-interval" yaml:"heartbeat-interval"`
	ServerSelectionTimeout *time.Duration `mapstructure:"server-selection-timeout" json:"server-selection-timeout" yaml:"server-selection-timeout"`
	RetryWrites            *bool          `mapstructure:"retry-writes" json:"retry-writes" yaml:"retry-writes"`
	RetryReads             *bool          `mapstructure:"retry-reads" json:"retry-reads" yaml:"retry-reads"`
	Compressor             []string       `mapstructure:"compressor" json:"compressor" yaml:"compressor"`
	ZlibLevel              *int           `mapstructure:"zlib-level" json:"zlib-level" yaml:"zlib-level"`
	ZstdLevel              *int           `mapstructure:"zstd-level" json:"zstd-level" yaml:"zstd-level"`
	MetricConfig           MetricConfig   `mapstructure:"metrics" json:"metrics" yaml:"metrics"`
	Aggregations           string         `mapstructure:"aggregations" json:"aggregations" yaml:"aggregations"`
}

type TLSConfig struct {
	CaLocation string `json:"ca-location" mapstructure:"ca-location" yaml:"ca-location"`
	SkipVerify bool   `json:"skip-verify" mapstructure:"skip-verify" yaml:"skip-verify"`
}

type Pool struct {
	MinConn               *uint64        `mapstructure:"min-conn" json:"min-conn" yaml:"min-conn"`
	MaxConn               *uint64        `mapstructure:"max-conn" json:"max-conn" yaml:"max-conn"`
	MaxWaitTime           *time.Duration `mapstructure:"max-wait-time" json:"max-wait-time" yaml:"max-wait-time"`
	MaxConnectionIdleTime *time.Duration `mapstructure:"max-conn-idle-time" json:"max-conn-idle-time" yaml:"max-conn-idle-time"`
	MaxConnecting         *uint64        `mapstructure:"max-connecting" json:"max-connecting" yaml:"max-connecting"`
}
type MetricConfig struct {
	Buckets struct {
		ConnectionTimeReady       *[]float64 `mapstructure:"connection-time-ready" json:"connection-time-ready" yaml:"connection-time-ready"`
		ConnectionPoolTimeAcquire *[]float64 `mapstructure:"connection-pool-time-acquire" json:"connection-pool-time-acquire" yaml:"connection-pool-time-acquire"`
	} `mapstructure:"buckets" json:"buckets" yaml:"buckets"`
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

func getPoolMonitor(poolMetric *poolMetric) *event.PoolMonitor {

	return &event.PoolMonitor{
		Event: func(e *event.PoolEvent) {
			//log.Debug().Str("type", e.Type).Str("duration", e.Duration.String()).Str("address", e.Address).Str("id", fmt.Sprint(e.ConnectionID)).Msg("event from mongo pool")

			attributes := attribute.String("address", e.Address)

			attributesSet := attribute.NewSet(attributes)

			switch e.Type {
			// Created when an operation successfully acquires a connection for execution.
			// Have duration
			case event.GetSucceeded:
				poolMetric.TimeToAcquireConnection.Record(context.Background(), e.Duration.Microseconds(), metric.WithAttributeSet(attributesSet))
				poolMetric.UsedConnection.Add(context.Background(), 1, metric.WithAttributeSet(attributesSet))
				break

			// Created when a connection is checked back into the pool after an operation is executed.
			// Do not have duration
			case event.ConnectionReturned:
				poolMetric.TotalReturnedConnection.Add(context.Background(), 1, metric.WithAttributeSet(attributesSet))
				poolMetric.UsedConnection.Add(context.Background(), -1, metric.WithAttributeSet(attributesSet))
				break

			// Created when a connection is created, but not necessarily when it is used for an operation.
			// Do not have duration
			case event.ConnectionCreated:
				// Connections created can be closed even if they do not reach the 'ready' state."
				poolMetric.AliveConnection.Add(context.Background(), 1, metric.WithAttributeSet(attributesSet))
				break

			// Created after a connection completes a handshake and is ready to be used for operations.
			// Have duration
			case event.ConnectionReady:
				poolMetric.TimeToReadyConnection.Record(context.Background(), e.Duration.Microseconds(), metric.WithAttributeSet(attributesSet))
				break

			// Created when a connection is closed.
			case event.ConnectionClosed:
				poolMetric.TotalCloseConnection.Add(context.Background(), 1, metric.WithAttributeSet(attributesSet))
				poolMetric.AliveConnection.Add(context.Background(), -1, metric.WithAttributeSet(attributesSet))
				break
			// Created when a connection pool is ready.
			// No connection seems to be created before this event
			case event.PoolReady:
				break

			// Created when an operation cannot acquire a connection for execution.
			case event.GetFailed:
				poolMetric.TotalFailedAcquireConnection.Add(context.Background(), 1, metric.WithAttributeSet(attributesSet))
				// ConnectionCheckOutStarted -> ConnectionCheckOutFailed quindi non serve se ascoltiamo ConnectionCheckedOut e ConnectionCheckedIn
				//poolMetric.UsedConnection.Add(context.Background(), -1, metric.WithAttributeSet(attributesSet))

				log.Error().Msg("Mongo Get Failed")
				break

			}
		},
	}
}

func configureMongo(cfg *Config, pollMetric *poolMetric) *options.ClientOptions {
	opts := options.Client()

	opts.Monitor = combineMonitors(
		otelmongo.NewMonitor(otelmongo.WithTracerProvider(otel.GetTracerProvider())),
		mongoprom.NewCommandMonitor(
			mongoprom.WithInstanceName(""),
			mongoprom.WithNamespace(""),
		),
	)
	opts.PoolMonitor = getPoolMonitor(pollMetric)

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
	if cfg.Pool.MaxConnecting != nil {
		opts.SetMaxConnecting(*cfg.Pool.MaxConnecting)
	}

}
