package mongo

import (
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
)

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
			continue // Puoi anche decidere di ritornare un errore se i tag sono obbligatori
		}

		_, ok := field.Tag.Lookup("omitempty")

		if ok && valField.IsZero() {
			continue
		}

		// Usa il nome del campo dal tag 'field' per il bson.M
		bsonFieldName := fieldNameTag

		// Gestisci diversi operatori
		switch operatorTag {
		case "$eq", "$ne", "$gt", "$gte", "$lt", "$lte":
			filter[bsonFieldName] = bson.M{operatorTag: fieldValue}
		case "$in", "$nin":
			// Verifica che il valore sia una slice per operatori $in e $nin
			if reflect.ValueOf(fieldValue).Kind() != reflect.Slice {
				return nil, fmt.Errorf("campo '%s' con operatore '%s' deve essere una slice", fieldNameTag, operatorTag)
			}
			filter[bsonFieldName] = bson.M{operatorTag: fieldValue}
		case "$exists":
			// Per $exists ci aspettiamo un booleano
			boolValue, ok := fieldValue.(bool)
			if !ok {
				return nil, fmt.Errorf("campo '%s' con operatore '$exists' deve essere un booleano", fieldNameTag)
			}
			filter[bsonFieldName] = bson.M{operatorTag: boolValue}
		default:
			return nil, fmt.Errorf("operatore '%s' non supportato per il campo '%s'", operatorTag, fieldNameTag)
		}
	}

	return filter, nil
}
