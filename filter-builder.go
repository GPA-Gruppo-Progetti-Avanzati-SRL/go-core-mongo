package mongo

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"maps"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
)

var operatorHandlers = map[string]func(string, interface{}) (bson.M, error){
	"$eq":     handleSimpleOperator,
	"$ne":     handleSimpleOperator,
	"$gt":     handleSimpleOperator,
	"$gte":    handleSimpleOperator,
	"$lt":     handleSimpleOperator,
	"$lte":    handleSimpleOperator,
	"$in":     handleArrayOperator,
	"$nin":    handleArrayOperator,
	"$exists": handleBoolOperator,
}

// buildFilter converte una struct con tag specifici in un bson.M per query MongoDB.
// La struct deve avere i campi taggati con:
// - `field:"nome_campo_mongodb"`:  Il nome del campo in MongoDB.
// - `operator:"$operatore"`: L'operatore MongoDB da usare (es. $eq, $in, $gt, $lt).
func buildFilter(inputStruct interface{}) (bson.M, error) {
	if inputStruct == nil {
		return nil, fmt.Errorf("input non puo essere nil")
	}
	val := reflect.ValueOf(inputStruct)
	typ := reflect.TypeOf(inputStruct)
	// Se è un puntatore, dereferenzialo
	if typ.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("input non può essere un puntatore nil")
		}
		val = val.Elem()
		typ = val.Type()
	}
	// Verifica che l'input sia una struct
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input non è una struct")
	}

	filter := bson.M{}

	// Itera attraverso i campi della struct
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i).Interface()
		valField := val.Field(i) // Get reflect.Value for IsZero() check

		// Ottieni i tag 'field' e 'operator'
		fieldNameTag := field.Tag.Get("field")
		operatorTag := field.Tag.Get("operator")

		// Se mancano i tag 'field' o 'operator', salta il campo
		if fieldNameTag == "" || operatorTag == "" {
			continue
		}

		_, ok := field.Tag.Lookup("omitempty")
		if ok && valField.IsZero() {
			continue
		}

		// Usa il nome del campo dal tag 'field' per il bson.M
		bsonFieldName := fieldNameTag

		handler, ok := operatorHandlers[operatorTag]
		if !ok {
			return nil, fmt.Errorf("operatore '%s' non supportato per il campo '%s'", operatorTag, fieldNameTag)
		}

		opFilter, err := handler(operatorTag, fieldValue)
		if err != nil {
			return nil, fmt.Errorf("errore per campo '%s' operatore '%s': %w", fieldNameTag, operatorTag, err)
		}
		previousFilter, ok := filter[bsonFieldName]

		if !ok {
			filter[bsonFieldName] = opFilter
		} else {
			maps.Copy(previousFilter.(bson.M), opFilter)
		}

	}
	if zerolog.GlobalLevel() < zerolog.InfoLevel {

		log.Debug().Msgf("mongo filter: %v", MongoFilterToJson(filter))
	}

	return filter, nil
}

func handleSimpleOperator(operator string, fieldValue interface{}) (bson.M, error) {
	return bson.M{operator: fieldValue}, nil
}

func handleArrayOperator(operator string, fieldValue interface{}) (bson.M, error) {
	if reflect.ValueOf(fieldValue).Kind() != reflect.Slice {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo slice", operator)
	}
	return bson.M{operator: fieldValue}, nil
}

func handleBoolOperator(operator string, fieldValue interface{}) (bson.M, error) {
	boolValue, ok := fieldValue.(bool)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo booleano", operator)
	}
	return bson.M{operator: boolValue}, nil
}

func MongoFilterToJson(filter any) string {

	mappa := bson.M{"filter": filter}

	value, err := bson.MarshalExtJSON(mappa, false, false)

	if err != nil {
		return ""
	}
	return string(value)

}
