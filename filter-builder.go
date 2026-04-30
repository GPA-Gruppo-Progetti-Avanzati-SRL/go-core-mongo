package coremongo

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type IFilter interface {
	GetFilterCollectionName(ctx context.Context) string
}

var operatorHandlers = map[string]func(string, interface{}) (bson.M, error){
	"$eq":          handleSimpleOperator,
	"$ne":          handleSimpleOperator,
	"$gt":          handleSimpleOperator,
	"$gte":         handleSimpleOperator,
	"$lt":          handleSimpleOperator,
	"$lte":         handleSimpleOperator,
	"$in":          handleArrayOperator,
	"$nin":         handleArrayOperator,
	"$all":         handleArrayOperator,
	"$exists":      handleBoolOperator,
	"$startswith":  handleStartsWithOperator,
	"$istartswith": handleIStartsWithOperator,
	"$endswith":    handleEndsWithOperator,
	"$iendswith":   handleIEndsWithOperator,
	"$contains":    handleContainsOperator,
	"$icontains":   handleIContainsOperator,
	"$regex":       handleRegexOperator,
	"$size":        handleSizeOperator,
}

// buildFilter converte una struct con tag specifici in un bson.M per query MongoDB.
// La struct deve avere i campi taggati con:
// - `field:"nome_campo_mongodb"`:  Il nome del campo in MongoDB.
// - `operator:"$operatore"`: L'operatore MongoDB da usare (es. $eq, $in, $gt, $lt).
func buildFilter(inputStruct IFilter) (bson.M, error) {
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
	if zerolog.GlobalLevel() < zerolog.DebugLevel {

		log.Trace().Msgf("mongo filter: %v", FilterToJson(filter))
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

func handleStartsWithOperator(operator string, fieldValue interface{}) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": "^" + strValue}, nil
}

func handleIStartsWithOperator(operator string, fieldValue interface{}) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": bson.Regex{Pattern: "^" + strValue, Options: "i"}}, nil
}

func handleEndsWithOperator(operator string, fieldValue interface{}) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": strValue + "$"}, nil
}

func handleIEndsWithOperator(operator string, fieldValue interface{}) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": bson.Regex{Pattern: strValue + "$", Options: "i"}}, nil
}

func handleContainsOperator(operator string, fieldValue interface{}) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": strValue}, nil
}

func handleIContainsOperator(operator string, fieldValue interface{}) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": bson.Regex{Pattern: strValue, Options: "i"}}, nil
}

func handleRegexOperator(operator string, fieldValue interface{}) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": strValue}, nil
}

func handleSizeOperator(operator string, fieldValue interface{}) (bson.M, error) {
	val := reflect.ValueOf(fieldValue)
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return bson.M{"$size": val.Int()}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return bson.M{"$size": val.Uint()}, nil
	default:
		return nil, fmt.Errorf("operatore '%s' richiede un valore intero", operator)
	}
}

func FilterToJson(filter any) string {

	mappa := bson.M{"filter": filter}

	value, err := bson.MarshalExtJSON(mappa, false, false)

	if err != nil {
		return ""
	}

	json, err := PrettyPrintJson(value)
	if err != nil {
		return ""
	}
	return json

}
