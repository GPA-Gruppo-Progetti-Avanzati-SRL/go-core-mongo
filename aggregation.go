package coremongo

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/rs/zerolog"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Aggregation struct {
	Name       string   `mapstructure:"name" json:"name" yaml:"name"`
	Collection string   `mapstructure:"collection" json:"collection" yaml:"collection"`
	Stages     []*Stage `mapstructure:"stages" json:"stages" yaml:"stages"`
}
type Stage struct {
	Key      string         `mapstructure:"key" json:"key" yaml:"key"`
	Operator string         `mapstructure:"operator" json:"operator" yaml:"operator"`
	Args     map[string]any `mapstructure:"args" json:"args" yaml:"args"`
}

// aggregations è il registry delle pipeline di un Service, indicizzato per il
// campo `name` dei file YAML. È per-istanza: due Service non si sovrascrivono.
type aggregations map[string]*Aggregation

var stageGenerators map[string]generateStage

// loadAggregations percorre dir in ricorsione e carica come pipeline ogni file
// .yaml/.yml trovato, a qualsiasi profondità: il nome della cartella non serve,
// quindi l'app passa la sua embed.FS così com'è.
//
// dir nil significa "nessuna aggregation": l'app non ha passato WithAggregations.
// Ogni altra anomalia è un errore, così l'app non parte con pipeline mancanti.
func loadAggregations(dir fs.FS) (aggregations, error) {
	if dir == nil {
		return nil, nil
	}
	stageGenerators = map[string]generateStage{

		"$skip":      simpleParams,
		"$limit":     simpleParams,
		"$project":   simpleArgs,
		"$sort":      sort,
		"$group":     simpleArgs,
		"$addFields": simpleArgs,
		"$match":     match,
		"$unionWith": unionWith,
	}
	regs := make(aggregations)
	err := fs.WalkDir(dir, ".", func(p string, d fs.DirEntry, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}
		if d.IsDir() {
			return nil
		}
		if ext := path.Ext(p); ext != ".yaml" && ext != ".yml" {
			return nil
		}

		content, errRead := fs.ReadFile(dir, p)
		if errRead != nil {
			return fmt.Errorf("aggregation %s: %w", p, errRead)
		}
		a := &Aggregation{}
		if errUm := yaml.Unmarshal(content, a); errUm != nil {
			return fmt.Errorf("aggregation %s: %w", p, errUm)
		}
		if a.Name == "" {
			return fmt.Errorf("aggregation %s: campo name mancante", p)
		}
		if _, dup := regs[a.Name]; dup {
			return fmt.Errorf("aggregation %s: nome %q già definito in un altro file", p, a.Name)
		}

		regs[a.Name] = a
		log.Info().Str("aggregation", a.Name).Str("file", p).Msg("aggregation loaded")
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(regs) == 0 {
		return nil, errors.New("aggregations: nessun file .yaml/.yml nella FS passata a WithAggregations")
	}
	return regs, nil
}

// pipeline genera la mongo.Pipeline di a, risolvendo i parametri per chiave di stage.
func (r aggregations) pipeline(a *Aggregation, params map[string]any) (mongo.Pipeline, *core.ApplicationError) {

	mp := make(mongo.Pipeline, 0)
	for _, stage := range a.Stages {

		fparams := params[stage.Key]
		gs, ok := stageGenerators[stage.Operator]
		if !ok {
			return nil, core.TechnicalError().WithCode("UNKNOWN Operator").WithMessage("operator " + stage.Operator + " is not supported")
		}
		s, errG := gs(r, stage.Operator, stage.Args, fparams)
		if errG != nil {
			return nil, errG
		}

		mp = append(mp, s)
	}
	return mp, nil

}

// generateStage genera un singolo stage. Il registry serve solo a unionWith, che
// compone per nome un'altra pipeline dello stesso Service.
type generateStage func(r aggregations, function string, args map[string]any, params any) (bson.D, *core.ApplicationError)

func unionWith(r aggregations, function string, args map[string]any, params any) (bson.D, *core.ApplicationError) {

	pipelineName, okP := args["pipeline"].(string)
	if !okP {
		return nil, core.TechnicalError().WithCode("").WithMessage(fmt.Sprintf("pipeline %s not found", pipelineName))
	}
	a, okA := r[pipelineName]
	if !okA {
		return nil, core.TechnicalError().WithCode("").WithMessage(fmt.Sprintf("aggregation %s not found", pipelineName))
	}

	var paramsCast map[string]any = nil

	// handle the case if the params is nil
	resultCast, ok := params.(map[string]any)
	if ok {
		paramsCast = resultCast
	}

	mp, err := r.pipeline(a, paramsCast)

	if err != nil {
		return nil, err
	}

	return bson.D{{Key: function, Value: bson.M{
		"coll":     a.Collection,
		"pipeline": mp,
	}}}, nil

}

func simpleParams(r aggregations, function string, args map[string]any, params any) (bson.D, *core.ApplicationError) {
	return bson.D{{Key: function, Value: params}}, nil
}

func simpleArgs(r aggregations, function string, args map[string]any, params any) (bson.D, *core.ApplicationError) {
	return bson.D{{Key: function, Value: args}}, nil
}
func match(r aggregations, function string, args map[string]any, params any) (bson.D, *core.ApplicationError) {
	if params == nil {
		return simpleArgs(r, function, args, params)
	}
	p, ok := params.(IFilter)
	if !ok {
		return nil, core.TechnicalError().WithCode("MON-FIL").WithMessage("Filtro non di tipo IFilter")
	}

	filterM, err := buildFilter(p)

	if err != nil {
		return nil, core.TechnicalError().WithCause(err)
	}
	return bson.D{{Key: function, Value: filterM}}, nil
}

func sort(r aggregations, function string, args map[string]any, params any) (bson.D, *core.ApplicationError) {
	sortBson := bson.D{}
	sortEl, ok := args["order"].([]any)
	if !ok {
		return nil, core.TechnicalError().WithCode("MON-SOR").WithMessage("order non trovato")
	}

	for _, sortField := range sortEl {
		sortFi, sok := sortField.(map[string]any)
		if !sok {
			return nil, core.TechnicalError().WithCode("MON-SOR").WithMessage("no sort structure")

		}

		sortC, cok := sortFi["field"].(string)
		if !cok {
			return nil, core.TechnicalError().WithCode("MON-SOR").WithMessage("no sort field in sort")

		}
		sortV, vok := sortFi["verse"].(string)
		if !vok {
			return nil, core.TechnicalError().WithCode("MON-SOR").WithMessage("no  sort verse in sort")

		}
		order := 1 // Default to ascending
		if sortV == "desc" {
			order = -1
		}
		sortBson = append(sortBson, bson.E{Key: sortC, Value: order})
	}

	return bson.D{{Key: function, Value: sortBson}}, nil
}

func (s *Service) ExecuteAggregation[T any](ctx context.Context, name string, params map[string]any, opts ...options.Lister[options.AggregateOptions]) ([]*T, *core.ApplicationError) {
	aggregation, ok := s.aggregations[name]
	if !ok {
		return nil, core.BusinessError().WithCode("NOT-FOUND").WithMessage(fmt.Sprintf("aggregation '%s' not found", name))
	}
	mp, err := s.aggregations.pipeline(aggregation, params)

	if err != nil {
		return nil, err
	}
	if zerolog.GlobalLevel() < zerolog.DebugLevel {
		value := PipelineToJson(mp)
		log.Trace().Str("pipeline", value).Msg("aggregation pipeline")
	}

	cur, errAgg := s.GetCollection(aggregation.Collection, "").Aggregate(ctx, mp, opts...)
	if errAgg != nil {
		if errors.Is(errAgg, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError().WithCause(errAgg)
		}
		return nil, core.TechnicalError().WithCode("MONGO-EXECAGGR").WithCause(errAgg)
	}
	defer func() {
		ccerr := cur.Close(ctx)
		if ccerr != nil {
			log.Error().Err(ccerr).Msg("close cursor error")
		}
	}()
	results := make([]*T, 0)
	if errCur := cur.All(ctx, &results); errCur != nil {
		return nil, core.TechnicalError().WithCode("MONGO-EXECAGGR-CUR").WithCause(errCur)
	}

	return results, nil
}
func PipelineToJson(pipeline mongo.Pipeline) string {
	// Ensure we never return an empty string so logs are not blank
	if len(pipeline) == 0 {
		return "[]"
	}
	// First attempt: wrap as bson.A to avoid top-level array writer issues
	// (mongo.Pipeline is []bson.D; we must copy into a []any-backed bson.A)
	arr := make(bson.A, 0, len(pipeline))
	for _, st := range pipeline {
		arr = append(arr, st)
	}
	if data, err := bson.MarshalExtJSON(arr, false, false); err == nil {
		return string(data)
	}
	// Fallback: marshal each stage individually and assemble a JSON array
	parts := make([]string, 0, len(pipeline))
	for _, stage := range pipeline {
		b, err := bson.MarshalExtJSON(stage, false, false)
		if err != nil {
			return "<pipeline-marshal-error: " + err.Error() + ">"
		}
		parts = append(parts, string(b))
	}
	// Join with commas into a JSON array
	return "[" + strings.Join(parts, ",") + "]"
}
