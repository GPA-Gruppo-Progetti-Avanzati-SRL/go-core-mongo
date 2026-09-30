package coremongo

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type IFilter interface {
	GetFilterCollectionName(ctx context.Context) string
}

var operatorHandlers = map[string]func(string, any) (bson.M, error){
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
	// Lo scheletro (nil, puntatore, struct, tag, omitempty) è core.TaggedFields, condiviso col
	// filter builder di go-core-sql. Prima il valore si leggeva prima dei tag, quindi un
	// campo non esportato qualsiasi faceva panicare reflect.
	fields, err := core.TaggedFields(inputStruct, "field", "operator")
	if err != nil {
		return nil, err
	}

	filter := bson.M{}

	for _, tf := range fields {
		fieldNameTag, operatorTag, fieldValue := tf.Key, tf.Op, tf.Value

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

func handleSimpleOperator(operator string, fieldValue any) (bson.M, error) {
	return bson.M{operator: fieldValue}, nil
}

func handleArrayOperator(operator string, fieldValue any) (bson.M, error) {
	if reflect.ValueOf(fieldValue).Kind() != reflect.Slice {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo slice", operator)
	}
	return bson.M{operator: fieldValue}, nil
}

func handleBoolOperator(operator string, fieldValue any) (bson.M, error) {
	boolValue, ok := fieldValue.(bool)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo booleano", operator)
	}
	return bson.M{operator: boolValue}, nil
}

func handleStartsWithOperator(operator string, fieldValue any) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": "^" + strValue}, nil
}

func handleIStartsWithOperator(operator string, fieldValue any) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": bson.Regex{Pattern: "^" + strValue, Options: "i"}}, nil
}

func handleEndsWithOperator(operator string, fieldValue any) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": strValue + "$"}, nil
}

func handleIEndsWithOperator(operator string, fieldValue any) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": bson.Regex{Pattern: strValue + "$", Options: "i"}}, nil
}

func handleContainsOperator(operator string, fieldValue any) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": strValue}, nil
}

func handleIContainsOperator(operator string, fieldValue any) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": bson.Regex{Pattern: strValue, Options: "i"}}, nil
}

func handleRegexOperator(operator string, fieldValue any) (bson.M, error) {
	strValue, ok := fieldValue.(string)
	if !ok {
		return nil, fmt.Errorf("operatore '%s' richiede un valore di tipo stringa", operator)
	}
	return bson.M{"$regex": strValue}, nil
}

func handleSizeOperator(operator string, fieldValue any) (bson.M, error) {
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
	return string(value)

}
