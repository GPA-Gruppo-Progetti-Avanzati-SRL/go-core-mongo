package coremongo

import (
	"context"
	"errors"
	"fmt"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ICollection interface {
	GetCollectionName() string
}

func GetObjectById[T ICollection](ctx context.Context, ms *mongolks.LinkedService, id string) (*T, *core.ApplicationError) {
	var result T

	collection := result.GetCollectionName()
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

	collection := filter.GetFilterCollectionName()
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
	collection := obj.GetCollectionName()
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
	collection := obj.GetCollectionName()
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalErrorWithError(errB)
	}
	cur, err := ms.GetCollection(collection, "").Find(ctx, filterB)
	if err != nil {
		return nil, core.TechnicalErrorWithError(err)
	}
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, core.TechnicalErrorWithError(errCur)
	}
	return results, nil

}

func GetObjectsByFilterSorted[T ICollection](ctx context.Context, ms *mongolks.LinkedService, filter IFilter, sort map[string]int) ([]*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName()
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalErrorWithError(errB)
	}
	findOptions := options.Find().SetSort(sort)
	cur, err := ms.GetCollection(collection, "").Find(ctx, filterB, findOptions)
	if err != nil {
		return nil, core.TechnicalErrorWithError(err)
	}
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, core.TechnicalErrorWithError(errCur)
	}
	return results, nil

}

func InsertOne(ctx context.Context, ms *mongolks.LinkedService, obj ICollection, opts ...*options.InsertOneOptions) *core.ApplicationError {

	collection := ms.GetCollection(obj.GetCollectionName(), "")
	res, errIns := collection.InsertOne(ctx, obj, opts...)
	if errIns != nil {
		return core.TechnicalErrorWithError(errIns)
	}
	if res.InsertedID == nil {
		return core.NotFoundError()
	}
	return nil
}

func InsertMany(ctx context.Context, ms *mongolks.LinkedService, objs []ICollection, opts ...*options.InsertManyOptions) *core.ApplicationError {
	collName := ""
	list := make([]interface{}, 0)
	for _, v := range objs {
		if collName == "" {
			collName = v.GetCollectionName()
		}
		if collName != v.GetCollectionName() {
			return core.TechnicalErrorWithCodeAndMessage("COLL-MIX", fmt.Sprintf("Get Collection Mix %s %s", collName, v.GetCollectionName()))
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

func UpdateOne(ctx context.Context, ms *mongolks.LinkedService, filter IFilter, update bson.M, opts ...*options.UpdateOptions) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.GetCollection(filter.GetFilterCollectionName(), "")
	res, err := collectionNotifiche.UpdateOne(ctx, filterB, update, opts...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(), err.Error())
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
	collectionNotifiche := ms.GetCollection(filter.GetFilterCollectionName(), "")
	res, err := collectionNotifiche.UpdateMany(ctx, filterB, update)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(), err.Error())
		return core.TechnicalErrorWithError(err)
	}
	if res.ModifiedCount != int64(len) {
		log.Error().Err(err).Msgf("Aggiornamento incoerente")
		return core.TechnicalErrorWithCodeAndMessage("MON-AGGINC", "aggiornamento incoerente")
	}
	return nil
}

func ReplaceOne(ctx context.Context, ms *mongolks.LinkedService, filter IFilter, obj ICollection, ro ...*options.ReplaceOptions) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.GetCollection(obj.GetCollectionName(), "")
	res, err := collectionNotifiche.ReplaceOne(ctx, filterB, obj, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile replace %s %s", obj.GetCollectionName(), err.Error())
		return core.TechnicalErrorWithError(err)
	}
	if res.ModifiedCount != 1 && res.UpsertedCount != 1 {
		log.Error().Err(err).Msgf("Aggiornamento incoerente")
		return core.TechnicalErrorWithCodeAndMessage("MON-AGGINC", "aggiornamento incoerente")
	}
	return nil
}
