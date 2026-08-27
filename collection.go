package coremongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/page"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

type ICollection interface {
	GetCollectionName(ctx context.Context) string
}

// codeCollectionNotFound è il codice applicativo per una collection richiesta ma non presente in
// collections: della config. mongolks.LinkedService.GetCollection non propaga errore in quel caso —
// logga e ritorna nil — quindi senza il controllo la prima chiamata sul *mongo.Collection nil
// andrebbe in panic invece di fallire con un ApplicationError.
const codeCollectionNotFound = "MONGO-COLL-NOTFOUND"

// collection risolve la collection e verifica che GetCollection non abbia ritornato nil, così
// ogni CRUD generico fallisce con un ApplicationError invece di panicare su una collection nil.
func (s *Service) collection(collectionId string, wc string) (*mongo.Collection, *core.ApplicationError) {
	coll := s.GetCollection(collectionId, wc)
	if coll == nil {
		return nil, core.TechnicalError().WithCode(codeCollectionNotFound).
			WithMessage(fmt.Sprintf("collection '%s' non configurata", collectionId))
	}
	return coll, nil
}

func (s *Service) GetObjectById[T ICollection](ctx context.Context, id string) (*T, *core.ApplicationError) {
	var result T

	collection := result.GetCollectionName(ctx)
	filter := bson.D{
		bson.E{Key: "_id", Value: id},
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	err := coll.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError().WithCause(err)
		}
		return nil, core.TechnicalError().WithCause(err)
	}
	return &result, nil

}

func (s *Service) CountDocuments(ctx context.Context, filter IFilter) (int64, *core.ApplicationError) {

	collection := filter.GetFilterCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return 0, core.TechnicalError().WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return 0, collErr
	}
	i, err := coll.CountDocuments(ctx, filterB)
	if err != nil {
		return 0, core.TechnicalError().WithCause(err)
	}
	return i, nil

}

func (s *Service) GetObjectByFilter[T ICollection](ctx context.Context, filter IFilter) (*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalError().WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	err := coll.FindOne(ctx, filterB).Decode(&obj)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, core.NotFoundError().WithCause(err)
		}
		return nil, core.TechnicalError().WithCause(err)
	}
	return &obj, nil

}

func (s *Service) GetObjectsByFilter[T ICollection](ctx context.Context, filter IFilter) ([]*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalError().WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	cur, err := coll.Find(ctx, filterB)
	if err != nil {
		return nil, core.TechnicalError().WithCode("MONGO-GOBF-ERRFIND").WithCause(err)
	}
	defer cur.Close(ctx)
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, core.TechnicalError().WithCode("MONGO-GOBF-ERRCUR").WithCause(errCur)
	}
	return results, nil

}

func (s *Service) GetObjectsByFilterSorted[T ICollection](ctx context.Context, filter IFilter, sort page.SortRequest) ([]*T, *core.ApplicationError) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalError().WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	findOptions := options.Find().SetSort(SortToBson(sort))
	cur, err := coll.Find(ctx, filterB, findOptions)
	if err != nil {
		return nil, core.TechnicalError().WithCode("MONGO-GOBFS-ERRFIND").WithCause(err)
	}
	defer cur.Close(ctx)
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, core.TechnicalError().WithCode("MONGO-GOBFS-ERRFIND").WithCause(errCur)
	}
	return results, nil

}

func (s *Service) InsertOne[T ICollection](ctx context.Context, obj T, opts ...options.Lister[options.InsertOneOptions]) (any, *core.ApplicationError) {

	collection, collErr := s.collection(obj.GetCollectionName(ctx), "")
	if collErr != nil {
		return nil, collErr
	}
	res, errIns := collection.InsertOne(ctx, obj, opts...)

	if errIns != nil {
		return nil, core.TechnicalError().WithCause(errIns)
	}
	if res.InsertedID == nil {
		return nil, core.NotFoundError()
	}
	return res.InsertedID, nil
}

func (s *Service) InsertMany[T ICollection](ctx context.Context, list []T, opts ...options.Lister[options.InsertManyOptions]) *core.ApplicationError {
	if len(list) == 0 {
		return nil
	}
	// Il nome della collection viene dal primo elemento, non da uno zero value di T:
	// con T puntatore o interfaccia `var obj T` è nil e GetCollectionName va in panic.
	name := list[0].GetCollectionName(ctx)
	collection, collErr := s.collection(name, "")
	if collErr != nil {
		return collErr
	}

	res, errIns := collection.InsertMany(ctx, list, opts...)
	if errIns != nil {
		return core.TechnicalError().WithCause(errIns)
	}
	if len(res.InsertedIDs) != len(list) {
		message := fmt.Sprintf("Mismatch insert %s requested %d vs inserted %d ", name, len(list), len(res.InsertedIDs))
		log.Error().Msg(message)
		return core.TechnicalError().WithCode("INSERT-MISMATCH").WithMessage(message)
	}
	return nil
}

func (s *Service) UpdateOne(ctx context.Context, filter IFilter, update bson.M, opts ...options.Lister[options.UpdateOneOptions]) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalError().WithCause(errB)
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.UpdateOne(ctx, filterB, update, opts...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return core.TechnicalError().WithCause(err)
	}
	if res.ModifiedCount != 1 && res.UpsertedCount != 1 {
		log.Error().Err(err).Msg("Aggiornamento incoerente")
		return core.TechnicalError().WithCode("MON-AGGINC").WithMessage("aggiornamento incoerente")
	}
	return nil
}

func (s *Service) UpdateMany(ctx context.Context, filter IFilter, update bson.M, len int) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalError().WithCause(errB)
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.UpdateMany(ctx, filterB, update)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return core.TechnicalError().WithCause(err)
	}
	if res.ModifiedCount != int64(len) {
		log.Error().Err(err).Msg("Aggiornamento incoerente")
		return core.TechnicalError().WithCode("MON-AGGINC").WithMessage("aggiornamento incoerente")
	}
	return nil
}

func (s *Service) ReplaceOne[T ICollection](ctx context.Context, filter IFilter, obj ICollection, ro ...options.Lister[options.ReplaceOptions]) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalError().WithCause(errB)
	}
	collectionNotifiche, collErr := s.collection(obj.GetCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.ReplaceOne(ctx, filterB, obj, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile replace %s %s", obj.GetCollectionName(ctx), err.Error())
		return core.TechnicalError().WithCause(err)
	}
	if res.ModifiedCount != 1 && res.UpsertedCount != 1 {
		log.Error().Err(err).Msg("Aggiornamento incoerente")
		return core.TechnicalError().WithCode("MON-AGGINC").WithMessage("aggiornamento incoerente")
	}
	return nil
}

func (s *Service) DeleteOne(ctx context.Context, filter IFilter, ro ...options.Lister[options.DeleteOneOptions]) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalError().WithCause(errB)
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.DeleteOne(ctx, filterB, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile rimuovere %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return core.TechnicalError().WithCause(err)
	}
	if res.DeletedCount == 0 {
		return core.NotFoundError()
	}
	if res.DeletedCount != 1 {
		log.Error().Err(err).Msg("Rimozione incoerente")
		return core.TechnicalError().WithCode("MON-AGGINC").WithMessage("rimozione incoerente")
	}

	return nil
}

func (s *Service) DeleteMany(ctx context.Context, filter IFilter, ro ...options.Lister[options.DeleteManyOptions]) *core.ApplicationError {

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return core.TechnicalError().WithCause(errB)
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	_, err := collectionNotifiche.DeleteMany(ctx, filterB, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile rimuovere %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return core.TechnicalError().WithCause(err)
	}

	return nil
}

func (s *Service) ExecTransaction(ctx context.Context, transaction func(ctx context.Context) error) *core.ApplicationError {
	wc := writeconcern.Majority()
	txnOptions := options.Transaction().SetWriteConcern(wc)
	// Starts a session on the client
	session, err := s.Db().Client().StartSession()
	if err != nil {
		return core.TechnicalError().WithCause(err)
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
		return core.TechnicalError().WithCause(err)
	}
	return nil
}

func (s *Service) GetIds(ctx context.Context, filter string, collectionName string, sort string, limit int) ([]string, *core.ApplicationError) {
	var filterMap map[string]any
	if err := json.Unmarshal([]byte(filter), &filterMap); err != nil {
		log.Error().Err(err).Msg("error unmarshal filter")
		return nil, core.TechnicalError().WithCode("PROPERTIES").WithMessage("error unmarshal filter").WithCause(err)
	}
	var sortMap map[string]int
	if sort != "" {
		if serr := json.Unmarshal([]byte(sort), &sortMap); serr != nil {
			log.Error().Err(serr).Msgf("error unmarshal sort: %s", serr.Error())
			return nil, core.TechnicalError().WithCode("PROPERTIES").WithMessage("error unmarshal sort").WithCause(serr)
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

	coll, collErr := s.collection(collectionName, "")
	if collErr != nil {
		return nil, collErr
	}
	cursor, err := coll.Find(ctx, filterM, findOptions)
	if err != nil {
		errMsg := fmt.Errorf("error Mongo: %s", err.Error())
		return nil, core.TechnicalError().WithCause(errMsg)
	}
	defer cursor.Close(ctx)

	var ids []string
	for cursor.Next(ctx) {
		var result struct {
			Id string `bson:"_id"` // Campo _id come stringa
		}
		if errDecode := cursor.Decode(&result); errDecode != nil {
			return nil, core.TechnicalError().WithCause(errDecode)
		}
		ids = append(ids, result.Id)
	}

	return ids, nil
}

func (s *Service) GetPageByFilter[T ICollection](ctx context.Context, filter IFilter, paging *page.Paging, opts ...options.Lister[options.FindOptions]) ([]T, *core.ApplicationError) {
	collection, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return nil, collErr
	}

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, core.TechnicalError().WithCause(errB)
	}

	totalItems, errCount := collection.CountDocuments(ctx, filterB)
	if errCount != nil {
		return nil, core.TechnicalError().WithCause(errCount)
	}

	paging.SetTotalItems(totalItems)
	offset, errP := paging.Paging()
	if errP != nil {
		return nil, errP
	}

	if offset >= 0 {
		opts = append(opts, options.Find().SetSkip(int64(offset)))
		opts = append(opts, options.Find().SetLimit(int64(paging.PageSize)))
	}

	cursor, errFind := collection.Find(ctx, filterB, opts...)
	if errFind != nil {
		return nil, core.TechnicalError().WithCause(errFind)
	}
	defer cursor.Close(ctx)

	var results []T
	if errDecode := cursor.All(ctx, &results); errDecode != nil {
		return nil, core.TechnicalError().WithCause(errDecode)
	}

	return results, nil
}

func (s *Service) GetSequence(ctx context.Context, sequenceCollection, sequenceName string) (int, *core.ApplicationError) {
	seqColl, collErr := s.collection(sequenceCollection, "")
	if collErr != nil {
		return 0, collErr
	}

	// Define the filter and update for the findAndModify equivalent
	filter := bson.M{"_id": sequenceName}
	update := bson.M{"$inc": bson.M{"sequence": 1}}

	// Set options to return the new document after update; upsert creates the record on first access
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After).
		SetProjection(bson.M{"sequence": 1, "_id": 0}).
		SetUpsert(true)

	// Perform the FindOneAndUpdate operation
	var result bson.M
	err := seqColl.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return 0, core.TechnicalError().WithCause(err)
	}

	if sequence, ok := result["sequence"].(int32); ok { // Assuming sequence is an int32
		return int(sequence), nil
	} else {
		return 0, core.TechnicalError().WithCode("SEQ-INV").WithMessage("sequence is not an integer")
	}

}

func (s *Service) UpdateSingleRecord(ctx context.Context, collectionName string, filterR any, updateR any) error {
	collectionRicorrenza, collErr := s.collection(collectionName, "")
	if collErr != nil {
		return collErr
	}
	resR, err := collectionRicorrenza.UpdateOne(ctx, filterR, updateR)
	if err != nil {
		log.Error().Err(err).Msg("Impossibile aggiornare")
		return err
	}
	if resR.ModifiedCount != 1 {
		log.Error().Err(err).Msgf("Aggiornamento %s incoerente", collectionName)
		return errors.New("aggiornamento incoerente " + collectionName)
	}
	return nil
}
