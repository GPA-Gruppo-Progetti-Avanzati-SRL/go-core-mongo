package mongo

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var Aggregations map[string]*Aggregation

type AggregationDirectory embed.FS
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

var stageGenerators map[string]GenerateStage

func init() {
	stageGenerators = map[string]GenerateStage{

		"$skip":      simpleParams,
		"$limit":     simpleParams,
		"$project":   simpleArgs,
		"$sort":      sort,
		"$group":     simpleArgs,
		"$match":     match,
		"$unionWith": unionWith,
	}
}
func LoadAggregations(aggregationFolder string, aggregationFiles embed.FS) {
	Aggregations = make(map[string]*Aggregation)
	dir, err := aggregationFiles.ReadDir(aggregationFolder)
	if err != nil {
		log.Fatal().Err(err).Msg("aggregationFolder")
	}
	for _, file := range dir {
		log.Info().Msgf("Loading Aggregation %s", file.Name())
		yamlFile, errRead := aggregationFiles.ReadFile(filepath.Join(aggregationFolder, file.Name()))
		if errRead != nil {
			log.Error().Err(errRead).Msg("aggregation read" + file.Name())
			continue
		}
		a := &Aggregation{}
		errUm := yaml.Unmarshal(yamlFile, &a)
		if errUm != nil {
			log.Error().Err(errUm).Msg("aggregation Unmarshal " + file.Name())
			continue
		}
		Aggregations[a.Name] = a
		log.Info().Msgf("Aggregation loaded %s", a.Name)
	}
}

func GenerateAggregation(a *Aggregation, params map[string]any) (mongo.Pipeline, *core.ApplicationError) {

	mp := make(mongo.Pipeline, 0)
	for _, stage := range a.Stages {

		fparams := params[stage.Key]
		gs, ok := stageGenerators[stage.Operator]
		if !ok {
			return nil, core.TechnicalErrorWithCodeAndMessage("UNKNOWN Operator", "operator "+stage.Operator+" is not supported")
		}
		s, errG := gs(stage.Operator, stage.Args, fparams)
		if errG != nil {
			return nil, errG
		}

		mp = append(mp, s)
	}
	return mp, nil

}

type GenerateStage func(function string, args map[string]interface{}, params any) (bson.D, *core.ApplicationError)

func unionWith(function string, args map[string]interface{}, params any) (bson.D, *core.ApplicationError) {

	pipelineName, okP := args["pipeline"].(string)
	if !okP {
		return nil, core.TechnicalErrorWithCodeAndMessage("", fmt.Sprintf("pipeline %s not found", pipelineName))
	}
	a, okA := Aggregations[pipelineName]
	if !okA {
		return nil, core.TechnicalErrorWithCodeAndMessage("", fmt.Sprintf("aggregation %s not found", pipelineName))
	}

	var paramsCast map[string]interface{} = nil

	// handle the case if the params is nil
	resultCast, ok := params.(map[string]interface{})
	if ok {
		paramsCast = resultCast
	}

	mp, err := GenerateAggregation(a, paramsCast)

	if err != nil {
		return nil, err
	}

	return bson.D{{Key: function, Value: bson.M{
		"coll":     a.Collection,
		"pipeline": mp,
	}}}, nil

}

func simpleParams(function string, args map[string]interface{}, params any) (bson.D, *core.ApplicationError) {
	return bson.D{{Key: function, Value: params}}, nil
}

func simpleArgs(function string, args map[string]interface{}, params any) (bson.D, *core.ApplicationError) {
	return bson.D{{Key: function, Value: args}}, nil
}
func match(function string, args map[string]interface{}, params any) (bson.D, *core.ApplicationError) {
	p, ok := params.(IFilter)
	if !ok {
		return nil, core.TechnicalErrorWithCodeAndMessage("MON-FIL", "Filtro non di tipo IFilter")
	}

	filterM, err := buildFilter(p)

	if err != nil {
		return nil, core.TechnicalErrorWithError(err)
	}
	return bson.D{{Key: function, Value: filterM}}, nil
}

func sort(function string, args map[string]interface{}, params any) (bson.D, *core.ApplicationError) {
	sortBson := bson.D{}
	sortEl, ok := args["order"].([]any)
	if !ok {
		return nil, core.TechnicalErrorWithCodeAndMessage("MON-SOR", "order non trovato")
	}

	for _, sortField := range sortEl {
		sortFi, sok := sortField.(map[string]interface{})
		if !sok {
			return nil, core.TechnicalErrorWithCodeAndMessage("MON-SOR", "no sort structure")

		}

		sortC, cok := sortFi["field"].(string)
		if !cok {
			return nil, core.TechnicalErrorWithCodeAndMessage("MON-SOR", "no sort field in sort")

		}
		sortV, vok := sortFi["verse"].(string)
		if !vok {
			return nil, core.TechnicalErrorWithCodeAndMessage("MON-SOR", "no  sort verse in sort")

		}
		order := 1 // Default to ascending
		if sortV == "desc" {
			order = -1
		}
		sortBson = append(sortBson, bson.E{Key: sortC, Value: order})
	}

	return bson.D{{Key: function, Value: sortBson}}, nil
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
	if zerolog.GlobalLevel() < zerolog.DebugLevel {
		value := PipelineToJson(mp)
		log.Trace().Msg(value)
	}

	cur, errAgg := ms.Database.Collection(aggregation.Collection).Aggregate(ctx, mp, opts...)
	if errAgg != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError()
		}
		return nil, core.TechnicalErrorWithError(errAgg)
	}
	return cur, nil
}

func ExecuteAggregation[T any](ctx context.Context, ms *Service, name string, params map[string]any, opts ...*options.AggregateOptions) ([]*T, *core.ApplicationError) {
	aggregation, ok := Aggregations[name]
	if !ok {
		return nil, core.BusinessErrorWithCodeAndMessage("NOT-FOUND", fmt.Sprintf("aggregation '%s' not found", name))
	}
	mp, err := GenerateAggregation(aggregation, params)

	if err != nil {
		return nil, err
	}
	if zerolog.GlobalLevel() < zerolog.DebugLevel {
		value := PipelineToJson(mp)
		log.Trace().Msg(value)
	}

	cur, errAgg := ms.Database.Collection(aggregation.Collection).Aggregate(ctx, mp, opts...)
	if errAgg != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError()
		}
		return nil, core.TechnicalErrorWithError(errAgg)
	}
	results := make([]*T, 0)
	if errCur := cur.All(ctx, &results); errCur != nil {
		return nil, core.TechnicalErrorWithError(errCur)
	}

	return results, nil
}
func PipelineToJson(pipeline interface{}) string {

	mappa := bson.M{"pipeline": pipeline}

	value, err := bson.MarshalExtJSON(mappa, false, false)
	if err != nil {
		return ""
	}
	json, err := PrettyPrintJson(value)
	if err != nil {
		return ""
	}
	return "\n" + json
}


func PrettyPrintJson(jsonStr []byte) (string, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, jsonStr, "", "  ")
	if err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
