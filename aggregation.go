package mongo

import (
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	filterbuilder "github.com/Kamran151199/mongo-filter-struct"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Aggregation struct {
	Name       string  `mapstructure:"name" json:"name" yaml:"name"`
	Collection string  `mapstructure:"collection" json:"collection" yaml:"collection"`
	Steps      []*Step `mapstructure:"steps" json:"steps" yaml:"steps"`
}

func (a *Aggregation) GenerateAggregation(params map[string]any) (mongo.Pipeline, *core.ApplicationError) {

	mp := make(mongo.Pipeline, 0)
	for _, step := range a.Steps {

		fparams, ok := params[step.Key]

		var s bson.D
		var errG *core.ApplicationError
		if !ok {
			s, errG = step.Generate(nil)
		} else {
			s, errG = step.Generate(fparams)
		}
		if errG != nil {
			return nil, errG
		}

		mp = append(mp, s)
	}
	return mp, nil

}

var builder = filterbuilder.NewBuilder()

type Step struct {
	Key      string         `mapstructure:"key" json:"key" yaml:"key"`
	Function string         `mapstructure:"function" json:"function" yaml:"function"`
	Args     map[string]any `mapstructure:"args" json:"args" yaml:"args"`
}

func (s *Step) Generate(params any) (bson.D, *core.ApplicationError) {

	switch s.Function {
	case "$limit", "$skip":
		return bson.D{{Key: s.Function, Value: params}}, nil
	case "$unionWith":
	case "$match":
		{
			match, errFilter := buildFilter(params)
			if errFilter != nil {
				return nil, errFilter
			}
			return bson.D{{Key: s.Function, Value: match}}, nil
		}

	case "$project", "$sort":
		return bson.D{{Key: s.Function, Value: s.Args}}, nil
	default:
		return nil, core.TechnicalErrorWithCodeAndMessage("UNKNOWN METHOD", "method"+s.Function+" is not supported")
	}
	return nil, core.TechnicalErrorWithCodeAndMessage("ISNH", "It should never happer")
}

func buildFilter(filter any) (bson.M, *core.ApplicationError) {

	filterM, err := builder.BuildQuery(filter)

	if err != nil {
		return nil, core.TechnicalErrorWithError(err)
	}
	return filterM, nil
}
