package mongo

import (
	"fmt"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Aggregation struct {
	Name       string  `mapstructure:"name" json:"name" yaml:"name"`
	Collection string  `mapstructure:"collection" json:"collection" yaml:"collection"`
	Steps      []*Step `mapstructure:"steps" json:"steps" yaml:"steps"`
}
type Step struct {
	Key      string         `mapstructure:"key" json:"key" yaml:"key"`
	Function string         `mapstructure:"function" json:"function" yaml:"function"`
	Args     map[string]any `mapstructure:"args" json:"args" yaml:"args"`
}

var stepGenerators map[string]GenerateStep

func init() {
	stepGenerators = map[string]GenerateStep{

		"$skip":      simpleParams,
		"$limit":     simpleParams,
		"$project":   simpleArgs,
		"$sort":      simpleArgs,
		"$match":     match,
		"$unionWith": unionWith,
	}
}

func GenerateAggregation(a *Aggregation, params map[string]any) (mongo.Pipeline, *core.ApplicationError) {

	mp := make(mongo.Pipeline, 0)
	for _, step := range a.Steps {

		fparams := params[step.Key]
		gs, ok := stepGenerators[step.Function]
		if !ok {
			return nil, core.TechnicalErrorWithCodeAndMessage("UNKNOWN METHOD", "method "+step.Function+" is not supported")
		}
		s, errG := gs(step.Function, step.Args, fparams)
		if errG != nil {
			return nil, errG
		}

		mp = append(mp, s)
	}
	return mp, nil

}

type GenerateStep func(function string, args map[string]interface{}, params any) (bson.D, *core.ApplicationError)

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
	filterM, err := buildFilter(params)

	if err != nil {
		return nil, core.TechnicalErrorWithError(err)
	}
	return bson.D{{Key: function, Value: filterM}}, nil
}
