package coremongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

type ICollection interface {
	GetCollectionName(ctx context.Context) string
}

func GetObjectById[T ICollection](ctx context.Context, ms *mongolks.LinkedService, id string) (*T, *core.ApplicationError) {
	var result T

	collection := result.GetCollectionName(ctx)
	filter := bson.D{
		bson.E{Key: "_id", Value: id},
	}
	err := ms.GetCollection(collection, "").FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError()
		}
		return nil, core.TechnicalErrorWithError(err)
	}
	return &result, nil

}

func CountDocuments(ctx context.Context, ms *mongolks.LinkedService, filter IFilter) (int64, *core.ApplicationError) {

	collection := filter.GetFilterCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return 0, core.TechnicalErrorWithError(errB)
	}
	i, err := ms.GetCollection(collection, "").CountDocuments(ctx, filterB)
	if err != nil {
		return 0, core.TechnicalErrorWithError(err)
	}
	return i, nil

}

func GetObjectByFilter[T ICollection](ctx context.Context, ms *mongolks.LinkedService, filter IFilter) (*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalErrorWithError(errB)
	}
	err := ms.GetCollection(collection, "").FindOne(ctx, filterB).Decode(&obj)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError()
		}
		return nil, core.TechnicalErrorWithError(err)
	}
	return &obj, nil

}

func GetObjectsByFilter[T ICollection](ctx context.Context, ms *mongolks.LinkedService, filter IFilter) ([]*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalErrorWithError(errB)
	}
	cur, err := ms.GetCollection(collection, "").Find(ctx, filterB)
	if err != nil {
		return nil, core.TechnicalErrorWithCodeAndMessage("MONGO-GOBF-ERRFIND", err.Error())
	}
	defer cur.Close(ctx)
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, core.TechnicalErrorWithCodeAndMessage("MONGO-GOBF-ERRCUR", errCur.Error())
	}
	return results, nil

}

func GetObjectsByFilterSorted[T ICollection](ctx context.Context, ms *mongolks.LinkedService, filter IFilter, sort map[string]int) ([]*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalErrorWithError(errB)
	}
	findOptions := options.Find().SetSort(sort)
	cur, err := ms.GetCollection(collection, "").Find(ctx, filterB, findOptions)
	if err != nil {
		return nil, core.TechnicalErrorWithCodeAndMessage("MONGO-GOBFS-ERRFIND", err.Error())
	}
	defer cur.Close(ctx)
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, core.TechnicalErrorWithCodeAndMessage("MONGO-GOBFS-ERRFIND", errCur.Error())
	}
	return results, nil

}

func InsertOne(ctx context.Context, ms *mongolks.LinkedService, obj ICollection, opts ...options.Lister[options.InsertOneOptions]) *core.ApplicationError {

	collection := ms.GetCollection(obj.GetCollectionName(ctx), "")
	res, errIns := collection.InsertOne(ctx, obj, opts...)
	if errIns != nil {
		return core.TechnicalErrorWithError(errIns)
	}
	if res.InsertedID == nil {
		return core.NotFoundError()
	}
	return nil
}

func InsertMany(ctx context.Context, ms *mongolks.LinkedService, objs []ICollection, opts ...options.Lister[options.InsertManyOptions]) *core.ApplicationError {
	collName := ""
	list := make([]interface{}, 0)
	for _, v := range objs {
		if collName == "" {
			collName = v.GetCollectionName(ctx)
		}
		if collName != v.GetCollectionName(ctx) {
			return core.TechnicalErrorWithCodeAndMessage("COLL-MIX", fmt.Sprintf("Get Collection Mix %s %s", collName, v.GetCollectionName(ctx)))
		}
		list = append(list, v)
	}

	collection := ms.GetCollection(collName, "")
	res, errIns := collection.InsertMany(ctx, list, opts...)
	if errIns != nil {
		return core.TechnicalErrorWithError(errIns)
	}
	if len(res.InsertedIDs) != len(objs) {
		message := fmt.Sprintf("Mismatch insert %s requested %d vs inserted %d ", collName, len(objs), len(res.InsertedIDs))
		log.Error().Msgf(message)
		return core.TechnicalErrorWithCodeAndMessage("INSERT-MISMATCH", message)
	}
	return nil
}

func UpdateOne(ctx context.Context, ms *mongolks.LinkedService, filter IFilter, update bson.M, opts ...options.Lister[options.UpdateOneOptions]) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.GetCollection(filter.GetFilterCollectionName(ctx), "")
	res, err := collectionNotifiche.UpdateOne(ctx, filterB, update, opts...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return core.TechnicalErrorWithError(err)
	}
	if res.ModifiedCount != 1 && res.UpsertedCount != 1 {
		log.Error().Err(err).Msgf("Aggiornamento incoerente")
		return core.TechnicalErrorWithCodeAndMessage("MON-AGGINC", "aggiornamento incoerente")
	}
	return nil
}

func UpdateMany(ctx context.Context, ms *mongolks.LinkedService, filter IFilter, update bson.M, len int) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.GetCollection(filter.GetFilterCollectionName(ctx), "")
	res, err := collectionNotifiche.UpdateMany(ctx, filterB, update)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return core.TechnicalErrorWithError(err)
	}
	if res.ModifiedCount != int64(len) {
		log.Error().Err(err).Msgf("Aggiornamento incoerente")
		return core.TechnicalErrorWithCodeAndMessage("MON-AGGINC", "aggiornamento incoerente")
	}
	return nil
}

func ReplaceOne(ctx context.Context, ms *mongolks.LinkedService, filter IFilter, obj ICollection, ro ...options.Lister[options.ReplaceOptions]) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.GetCollection(obj.GetCollectionName(ctx), "")
	res, err := collectionNotifiche.ReplaceOne(ctx, filterB, obj, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile replace %s %s", obj.GetCollectionName(ctx), err.Error())
		return core.TechnicalErrorWithError(err)
	}
	if res.ModifiedCount != 1 && res.UpsertedCount != 1 {
		log.Error().Err(err).Msgf("Aggiornamento incoerente")
		return core.TechnicalErrorWithCodeAndMessage("MON-AGGINC", "aggiornamento incoerente")
	}
	return nil
}

func ExecTransaction(ctx context.Context, ms *mongolks.LinkedService, transaction func(ctx context.Context) error) *core.ApplicationError {
	wc := writeconcern.Majority()
	txnOptions := options.Transaction().SetWriteConcern(wc)
	// Starts a session on the client
	session, err := ms.Db().Client().StartSession()
	if err != nil {
		return core.TechnicalErrorWithError(err)
	}

	// Defers ending the session after the transaction is committed or ended
	defer session.EndSession(ctx)

	// Esecuzione della transazione
	err = mongo.WithSession(ctx, session, func(sessCtx context.Context) error {
		// Inizia la transazione
		if errSt := session.StartTransaction(txnOptions); errSt != nil {
			return errSt
		}

		// Esegue la transazione con il callback
		if errT := transaction(sessCtx); errT != nil {
			session.AbortTransaction(sessCtx) // Rollback
			return errT
		}

		// Commit della transazione
		return session.CommitTransaction(sessCtx)
	})
	if err != nil {
		return core.TechnicalErrorWithError(err)
	}
	return nil
}

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
