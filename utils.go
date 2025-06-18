package coremongo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func GetIds(ctx context.Context, ms *mongolks.LinkedService, filter string, collectionName string, sort string, limit int) ([]string, *core.ApplicationError) {
	var filterMap map[string]interface{}
	if err := json.Unmarshal([]byte(filter), &filterMap); err != nil {
		log.Error().Err(err).Msgf("error unmarshal filter")
		return nil, core.TechnicalErrorWithCodeAndMessage("PROPERTIES", "error unmarshal filter")
	}
	var sortMap map[string]int
	if sort != "" {
		if serr := json.Unmarshal([]byte(sort), &sortMap); serr != nil {
			log.Error().Err(serr).Msgf("error unmarshal sort", serr.Error())
			return nil, core.TechnicalErrorWithCodeAndMessage("PROPERTIES", "error unmarshal sort")
		}
	}

	// Converti eventuali stringhe ISO 8601 in oggetti time.Time
	filterMap = convertDates(filterMap)

	// Converti il filtro finale in bson.M
	filterM := bson.M(filterMap)

	projection := bson.M{"_id": 1} // Includi solo il campo _id
	findOptions := options.Find().SetProjection(projection).SetLimit(int64(limit))
	if sort != "" {
		findOptions = findOptions.SetSort(sortMap)
	}

	cursor, err := ms.GetCollection(collectionName, "").Find(ctx, filterM, findOptions)
	if err != nil {
		errMsg := fmt.Errorf("error Mongo: %s", err.Error())
		return nil, core.TechnicalErrorWithError(errMsg)
	}
	defer cursor.Close(ctx)

	var ids []string
	for cursor.Next(ctx) {
		var result struct {
			Id string `bson:"_id"` // Campo _id come stringa
		}
		if errDecode := cursor.Decode(&result); errDecode != nil {
			return nil, core.TechnicalErrorWithError(errDecode)
		}
		ids = append(ids, result.Id)
	}

	return ids, nil
}

func GetSequence(ctx context.Context, ms *mongolks.LinkedService, sequenceCollection, numeroOrdineSequenceName string) (int, *core.ApplicationError) {
	seqColl := ms.GetCollection(sequenceCollection, "")

	// Define the filter and update for the findAndModify equivalent
	filter := bson.M{"_id": numeroOrdineSequenceName}
	update := bson.M{"$inc": bson.M{"sequence": 1}}

	// Set options to return the new document after update
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After).
		SetProjection(bson.M{"sequence": 1, "_id": 0})

	// Perform the FindOneAndUpdate operation
	var result bson.M
	err := seqColl.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return 0, core.TechnicalErrorWithError(err)
	}

	if sequence, ok := result["sequence"].(int32); ok { // Assuming sequence is an int32
		return int(sequence), nil
	} else {
		return 0, core.TechnicalErrorWithCodeAndMessage("SEQ-INV", "sequence is not an integer")
	}

}

func UpdateSingleRecord(ctx context.Context, ms *mongolks.LinkedService, collectionName string, filterR interface{}, updateR interface{}) error {
	collectionRicorrenza := ms.GetCollection(collectionName, "")
	resR, err := collectionRicorrenza.UpdateOne(ctx, filterR, updateR)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare")
		return err
	}
	if resR.ModifiedCount != 1 {
		log.Error().Err(err).Msgf("Aggiornamento %s incoerente", collectionName)
		return errors.New("aggiornamento incoerente " + collectionName)
	}
	return nil
}

func convertDates(input map[string]interface{}) map[string]interface{} {

	for key, value := range input {
		if value == "CURRENT_TIMESTAMP" {
			value = time.Now()
		}
		switch v := value.(type) {
		case time.Time:
			input[key] = value

		case map[string]interface{}:
			// Ricorsione: esplora i livelli interni del documento
			input[key] = convertDates(v)
		}
	}
	return input
}

func PrettyPrintJson(jsonStr []byte) (string, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, jsonStr, "", "  ")
	if err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
