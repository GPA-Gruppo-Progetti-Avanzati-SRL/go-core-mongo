package mongo

import (
	"context"
	"errors"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

func CountDocuments[T ICollection](ctx context.Context, ms *Service, filter any) (int64, *core.ApplicationError) {
	var result T
	collection := result.GetCollectionName()
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

func GetObjectByFilter[T ICollection](ctx context.Context, ms *Service, filter any) (*T, *core.ApplicationError) {
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

func GetObjectsByFilter[T ICollection](ctx context.Context, ms *Service, filter any) ([]*T, *core.ApplicationError) {
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
