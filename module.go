package coremongo

import (
	"io/fs"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	authcore "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/authorization"
	mongoauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo/authorization"
)

// Option configura Module.
type Option func(*moduleOptions)

type moduleOptions struct {
	modes         []string
	aggregations  fs.FS
	authorization bool
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

// WithAuthorization fornisce l'authorization.Authorizer di go-core-app alimentato dalla
// collection ACL su Mongo (LUT con refresh periodico).
//
// Sostituisce il core.ProvideAs[authorization.Authorizer](mongoauth.NewAuthorizationLut) che
// l'app faceva a mano in main.go: così l'app non importa go-core-app/authorization solo per
// nominare un tipo, e soprattutto la LUT è gate-ata dagli stessi modes del Module — a mano era
// facile scordarselo e ritrovarsi la LUT che interroga Mongo anche in un processo worker.
//
//	coremongo.Module(&cfg.Mongo,
//	    coremongo.WithAggregations(aggregationFiles),
//	    coremongo.WithAuthorization())
func WithAuthorization() Option {
	return func(o *moduleOptions) { o.authorization = true }
}

// aggregationSource porta la FS delle aggregation dal Module al costruttore.
// È supplita solo se l'app passa WithAggregations: in newService è quindi una
// dipendenza optional (assente ⇒ servizio senza aggregation).
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
		if o.aggregations != nil {
			core.Supply(aggregationSource{dir: o.aggregations}, o.modes...)
		}
		core.Provide(newService, o.modes...)

		// La LUT dell'ACL dichiara la sua dipendenza come mongoauth.Collections (interfaccia a
		// un metodo): è ciò che tiene il subpackage indipendente dal root e quindi importabile
		// da qui. L'adapter è fornito sempre — fx lo costruisce solo se qualcuno lo chiede —
		// così continua a risolvere anche il wiring a mano di mongoauth.NewAuthorizationLut.
		core.Provide(func(s *Service) mongoauth.Collections { return s }, o.modes...)
		if o.authorization {
			core.ProvideAs[authcore.Authorizer](mongoauth.NewAuthorizationLut, o.modes...)
		}
	})
}
