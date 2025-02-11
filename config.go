package mongo

import (
	"time"
)

type Config struct {
	Server        string `yaml:"server" mapstructure:"server" json:"server"`
	Database      string `yaml:"database" mapstructure:"database" json:"database"`
	Username      string `yaml:"username" mapstructure:"username" json:"username"`
	Password      string `yaml:"password" mapstructure:"password" json:"password"`
	AuthMechanism string `mapstructure:"authMechanism" json:"authMechanism" yaml:"authMechanism"`
	AuthDB        string `yaml:"authdb" mapstructure:"authdb" json:"authdb"`
	Pool          *struct {
		MinConn               *uint64        `mapstructure:"min-conn" json:"min-conn" yaml:"min-conn"`
		MaxConn               *uint64        `mapstructure:"max-conn" json:"max-conn" yaml:"max-conn"`
		MaxWaitTime           *time.Duration `mapstructure:"max-wait-time" json:"max-wait-time" yaml:"max-wait-time"`
		MaxConnectionIdleTime *time.Duration `mapstructure:"max-conn-idle-time" json:"max-conn-idle-time" yaml:"max-conn-idle-time"`
		MaxConnecting         *uint64        `mapstructure:"max-connecting" json:"max-connecting" yaml:"max-connecting"`
	}
	WriteConcern           string         `mapstructure:"write-concern" json:"write-concern" yaml:"write-concern"`
	ReadConcern            string         `mapstructure:"read-concern" json:"read-concern" yaml:"read-concern"`
	OperationTimeout       *time.Duration `mapstructure:"operation-timeout" json:"operation-timeout" yaml:"operation-timeout"`
	SecurityProtocol       string         `mapstructure:"security-protocol" json:"security-protocol" yaml:"security-protocol"`
	TLS                    TLSConfig      `json:"tls" mapstructure:"tls" yaml:"tls"`
	HeartbeatInterval      *time.Duration `mapstructure:"heartbeat-interval" json:"heartbeat-interval" yaml:"heartbeat-interval"`
	ServerSelectionTimeout *time.Duration `mapstructure:"server-selection-timeout" json:"server-selection-timeout" yaml:"server-selection-timeout"`
	RetryWrites            *bool          `mapstructure:"reatry-writes" json:"reatry-writes" yaml:"reatry-writes"`
	RetryReads             *bool          `mapstructure:"reatry-reads" json:"reatry-reads" yaml:"reatry-reads"`
	Compressor             []string       `mapstructure:"compressor" json:"compressor" yaml:"compressor"`
	ZlibLevel              *int           `mapstructure:"zlib-level" json:"zlib-level" yaml:"zlib-level"`
	ZstdLevel              *int           `mapstructure:"zstd-level" json:"zstd-level" yaml:"zstd-level"`
	MetricConfig           MetricConfig   `mapstructure:"metrics" json:"metrics" yaml:"metrics"`
	Aggregations           []*Aggregation `mapstructure:"aggregations" json:"aggregations" yaml:"aggregations"`
}

type TLSConfig struct {
	CaLocation string `json:"ca-location" mapstructure:"ca-location" yaml:"ca-location"`
	SkipVerify bool   `json:"skip-verify" mapstructure:"skip-verify" yaml:"skip-verify"`
}

type MetricConfig struct {
	Buckets struct {
		ConnectionTimeReady       *[]float64 `mapstructure:"connection-time-ready" json:"connection-time-ready" yaml:"connection-time-ready"`
		ConnectionPoolTimeAcquire *[]float64 `mapstructure:"connection-pool-time-acquire" json:"connection-pool-time-acquire" yaml:"connection-pool-time-acquire"`
	} `mapstructure:"buckets" json:"buckets" yaml:"buckets"`
}
