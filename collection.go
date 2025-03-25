package mongo

import (
	"context"
	"errors"
	"fmt"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ICollection interface {
	GetCollectionName() string
}

func GetObjectById[T ICollection](ctx context.Context, ms *Service, id string) (*T, *core.ApplicationError) {
	var result T

	collection := result.GetCollectionName()
	filter := bson.D{
		bson.E{Key: "_id", Value: id},
	}
	err := ms.Database.Collection(collection).FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError()
		}
		return nil, core.TechnicalErrorWithError(err)
	}
	return &result, nil

}

func (ms *Service) CountDocuments(ctx context.Context, filter IFilter) (int64, *core.ApplicationError) {

	collection := filter.GetFilterCollectionName()
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return 0, core.TechnicalErrorWithError(errB)
	}
	i, err := ms.Database.Collection(collection).CountDocuments(ctx, filterB)
	if err != nil {
		return 0, core.TechnicalErrorWithError(err)
	}
	return i, nil

}

func GetObjectByFilter[T ICollection](ctx context.Context, ms *Service, filter IFilter) (*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName()
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalErrorWithError(errB)
	}
	err := ms.Database.Collection(collection).FindOne(ctx, filterB).Decode(&obj)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError()
		}
		return nil, core.TechnicalErrorWithError(err)
	}
	return &obj, nil

}

func GetObjectsByFilter[T ICollection](ctx context.Context, ms *Service, filter IFilter) ([]*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName()
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalErrorWithError(errB)
	}
	cur, err := ms.Database.Collection(collection).Find(ctx, filterB)
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

func (ms *Service) InsertOne(ctx context.Context, obj ICollection) *core.ApplicationError {

	collection := ms.Database.Collection(obj.GetCollectionName())
	res, errIns := collection.InsertOne(ctx, obj)
	if errIns != nil {
		return core.TechnicalErrorWithError(errIns)
	}
	if res.InsertedID == nil {
		return core.NotFoundError()
	}
	return nil
}

func (ms *Service) InsertMany(ctx context.Context, objs []ICollection, opts *options.InsertManyOptions) *core.ApplicationError {
	collName := ""
	list := make([]interface{}, len(objs))
	for i, v := range objs {
		list[i] = v
		if collName != "" {
			collName = v.GetCollectionName()
		}
		if collName != v.GetCollectionName() {
			return core.TechnicalErrorWithCodeAndMessage("COLL-MIX", fmt.Sprintf("Get Collection Mix %s %s", collName, v.GetCollectionName()))
		}
	}

	collection := ms.Database.Collection(collName)
	res, errIns := collection.InsertMany(ctx, list, opts)
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

func (ms *Service) UpdateOne(ctx context.Context, filter IFilter, update bson.M) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.Database.Collection(filter.GetFilterCollectionName())
	res, err := collectionNotifiche.UpdateOne(ctx, filterB, update)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(), err.Error())
		return core.TechnicalErrorWithError(err)
	}
	if res.ModifiedCount != 1 {
		log.Error().Err(err).Msgf("Aggiornamento incoerente")
		return core.TechnicalErrorWithCodeAndMessage("MON-AGGINC", "aggiornamento incoerente")
	}
	return nil
}

func (ms *Service) UpdateMany(ctx context.Context, filter IFilter, update bson.M, len int) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.Database.Collection(filter.GetFilterCollectionName())
	res, err := collectionNotifiche.UpdateOne(ctx, filterB, update)
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

func (ms *Service) ReplaceOne(ctx context.Context, filter IFilter, obj ICollection, ro *options.ReplaceOptions) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalErrorWithError(errB)
	}
	collectionNotifiche := ms.Database.Collection(obj.GetCollectionName())
	res, err := collectionNotifiche.ReplaceOne(ctx, filterB, obj, ro)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile replace %s %s", obj.GetCollectionName(), err.Error())
		return core.TechnicalErrorWithError(err)
	}
	if res.ModifiedCount != 1 {
		log.Error().Err(err).Msgf("Aggiornamento incoerente")
		return core.TechnicalErrorWithCodeAndMessage("MON-AGGINC", "aggiornamento incoerente")
	}
	return nil
}
