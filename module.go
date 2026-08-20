package coremongo

import (
	"io/fs"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
)

// Option configura Module.
type Option func(*moduleOptions)

type moduleOptions struct {
	modes        []string
	aggregations fs.FS
}

// WithModes limita la registrazione ai core.Mode indicati (nessuno = sempre).
func WithModes(modes ...string) Option {
	return func(o *moduleOptions) { o.modes = modes }
}

// WithAggregations registra le pipeline di aggregation contenute in dir.
//
// Si passa direttamente la variabile del //go:embed dell'app (embed.FS implementa
// fs.FS): la FS è percorsa in ricorsione e ogni file .yaml/.yml è una pipeline,
// indicizzata per il suo campo `name`. Il nome della cartella non serve.
//
//	//go:embed aggregations
//	var aggregationFiles embed.FS
//
//	coremongo.Module(&cfg.Mongo, coremongo.WithAggregations(aggregationFiles))
func WithAggregations(dir fs.FS) Option {
	return func(o *moduleOptions) { o.aggregations = dir }
}

// aggregationSource porta la FS delle aggregation dal Module al costruttore.
// È supplita sempre, anche vuota, così newService non ha dipendenze optional.
type aggregationSource struct {
	dir fs.FS
}

// Module wira il servizio Mongo nell'applicazione fx: supplisce la Config e
// fornisce *Service, unico handle Mongo per l'applicazione.
//
// È l'unico entry-point: il costruttore non è esportato e l'app non deve fare
// core.Supply/core.Provide a mano.
//
//	coremongo.Module(&cfg.Mongo)
//	coremongo.Module(&cfg.Mongo,
//	    coremongo.WithAggregations(aggregationFiles),
//	    coremongo.WithModes(engine.Api, engine.Batch))
func Module(cfg *Config, opts ...Option) {
	var o moduleOptions
	for _, opt := range opts {
		opt(&o)
	}

	core.Module("mongo", func() {
		core.Supply(cfg, o.modes...)
		core.Supply(aggregationSource{dir: o.aggregations}, o.modes...)
		core.Provide(newService, o.modes...)
	})
}
