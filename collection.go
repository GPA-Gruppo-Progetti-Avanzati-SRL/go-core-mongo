package coremongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/page"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo/mongoutil"
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
// andrebbe in panic invece di fallire con un core.Error.

// collection risolve la collection e verifica che GetCollection non abbia ritornato nil, così
// ogni CRUD generico fallisce con un core.Error invece di panicare su una collection nil.
func (s *Service) collection(collectionId string, wc string) (*mongo.Collection, *core.Error) {
	coll := s.GetCollection(collectionId, wc)
	if coll == nil {
		return nil, errs.Tech(CodeCollectionNotFound).
			WithMessage(fmt.Sprintf("collection '%s' non configurata", collectionId))
	}
	return coll, nil
}

func (s *Service) GetObjectById[T ICollection](ctx context.Context, id string) (*T, *core.Error) {
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
			return nil, errs.NotFound().WithCause(err)
		}
		return nil, errs.Tech(CodeFindOne).WithCause(err)
	}
	return &result, nil

}

func (s *Service) CountDocuments(ctx context.Context, filter IFilter) (int64, *core.Error) {

	collection := filter.GetFilterCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return 0, errs.Tech(CodeFilter).WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return 0, collErr
	}
	i, err := coll.CountDocuments(ctx, filterB)
	if err != nil {
		return 0, errs.Tech(CodeCount).WithCause(err)
	}
	return i, nil

}

func (s *Service) GetObjectByFilter[T ICollection](ctx context.Context, filter IFilter) (*T, *core.Error) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, errs.Tech(CodeFilter).WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	err := coll.FindOne(ctx, filterB).Decode(&obj)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errs.NotFound().WithCause(err)
		}
		return nil, errs.Tech(CodeFindOne).WithCause(err)
	}
	return &obj, nil

}

func (s *Service) GetObjectsByFilter[T ICollection](ctx context.Context, filter IFilter) ([]*T, *core.Error) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, errs.Tech(CodeFilter).WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	cur, err := coll.Find(ctx, filterB)
	if err != nil {
		return nil, errs.Tech(CodeGetObjectsFind).WithCause(err)
	}
	defer mongoutil.CloseCursor(ctx, cur, "GetObjectsByFilter")
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, errs.Tech(CodeGetObjectsCursor).WithCause(errCur)
	}
	return results, nil

}

func (s *Service) GetObjectsByFilterSorted[T ICollection](ctx context.Context, filter IFilter, sort page.SortRequest) ([]*T, *core.Error) {
	var obj T
	collection := obj.GetCollectionName(ctx)
	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, errs.Tech(CodeFilter).WithCause(errB)
	}
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	// In bson una chiave `$...` è un operatore: il campo di sort, che di solito arriva da un query
	// param, dev'essere un identificatore.
	if err := sort.Validate(); err != nil {
		return nil, errs.Business(CodeSort).WithCause(err)
	}
	findOptions := options.Find().SetSort(SortToBson(sort))
	cur, err := coll.Find(ctx, filterB, findOptions)
	if err != nil {
		return nil, errs.Tech(CodeGetObjectsSorted).WithCause(err)
	}
	defer mongoutil.CloseCursor(ctx, cur, "GetObjectsByFilterSorted")
	results := make([]*T, 0)
	errCur := cur.All(ctx, &results)
	if errCur != nil {
		return nil, errs.Tech(CodeGetObjectsSorted).WithCause(errCur)
	}
	return results, nil

}

func (s *Service) InsertOne[T ICollection](ctx context.Context, obj T, opts ...options.Lister[options.InsertOneOptions]) (any, *core.Error) {

	collection, collErr := s.collection(obj.GetCollectionName(ctx), "")
	if collErr != nil {
		return nil, collErr
	}
	res, errIns := collection.InsertOne(ctx, obj, opts...)

	if errIns != nil {
		return nil, errs.Tech(CodeInsert).WithCause(errIns)
	}
	if res.InsertedID == nil {
		return nil, errs.Tech(CodeInsertNoID).WithMessage("insert eseguita ma senza InsertedID")
	}
	return res.InsertedID, nil
}

func (s *Service) InsertMany[T ICollection](ctx context.Context, list []T, opts ...options.Lister[options.InsertManyOptions]) *core.Error {
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
		return errs.Tech(CodeInsert).WithCause(errIns)
	}
	if len(res.InsertedIDs) != len(list) {
		message := fmt.Sprintf("Mismatch insert %s requested %d vs inserted %d ", name, len(list), len(res.InsertedIDs))
		log.Error().Msg(message)
		return errs.Tech(CodeInsertMismatch).WithMessage(message)
	}
	return nil
}

func (s *Service) UpdateOne(ctx context.Context, filter IFilter, update bson.M, opts ...options.Lister[options.UpdateOneOptions]) *core.Error {

	filterB, appErr := buildWriteFilter(filter)
	if appErr != nil {
		return appErr
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.UpdateOne(ctx, filterB, update, opts...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return errs.Tech(CodeUpdate).WithCause(err)
	}
	if res.ModifiedCount != 1 && res.UpsertedCount != 1 {
		log.Error().Err(err).Msg("Aggiornamento incoerente")
		return errs.Tech(CodeInconsistent).WithMessage("aggiornamento incoerente")
	}
	return nil
}

func (s *Service) UpdateMany(ctx context.Context, filter IFilter, update bson.M, len int) *core.Error {

	filterB, appErr := buildWriteFilter(filter)
	if appErr != nil {
		return appErr
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.UpdateMany(ctx, filterB, update)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile aggiornare %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return errs.Tech(CodeUpdate).WithCause(err)
	}
	if res.ModifiedCount != int64(len) {
		log.Error().Err(err).Msg("Aggiornamento incoerente")
		return errs.Tech(CodeInconsistent).WithMessage("aggiornamento incoerente")
	}
	return nil
}

func (s *Service) ReplaceOne[T ICollection](ctx context.Context, filter IFilter, obj ICollection, ro ...options.Lister[options.ReplaceOptions]) *core.Error {

	filterB, appErr := buildWriteFilter(filter)
	if appErr != nil {
		return appErr
	}
	collectionNotifiche, collErr := s.collection(obj.GetCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.ReplaceOne(ctx, filterB, obj, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile replace %s %s", obj.GetCollectionName(ctx), err.Error())
		return errs.Tech(CodeReplace).WithCause(err)
	}
	if res.ModifiedCount != 1 && res.UpsertedCount != 1 {
		log.Error().Err(err).Msg("Aggiornamento incoerente")
		return errs.Tech(CodeInconsistent).WithMessage("aggiornamento incoerente")
	}
	return nil
}

func (s *Service) DeleteOne(ctx context.Context, filter IFilter, ro ...options.Lister[options.DeleteOneOptions]) *core.Error {

	filterB, appErr := buildWriteFilter(filter)
	if appErr != nil {
		return appErr
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	res, err := collectionNotifiche.DeleteOne(ctx, filterB, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile rimuovere %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return errs.Tech(CodeDelete).WithCause(err)
	}
	if res.DeletedCount == 0 {
		return errs.NotFound()
	}
	if res.DeletedCount != 1 {
		log.Error().Err(err).Msg("Rimozione incoerente")
		return errs.Tech(CodeInconsistent).WithMessage("rimozione incoerente")
	}

	return nil
}

func (s *Service) DeleteMany(ctx context.Context, filter IFilter, ro ...options.Lister[options.DeleteManyOptions]) *core.Error {

	filterB, appErr := buildWriteFilter(filter)
	if appErr != nil {
		return appErr
	}
	collectionNotifiche, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return collErr
	}
	_, err := collectionNotifiche.DeleteMany(ctx, filterB, ro...)
	if err != nil {
		log.Error().Err(err).Msgf("Impossibile rimuovere %s %s", filter.GetFilterCollectionName(ctx), err.Error())
		return errs.Tech(CodeDelete).WithCause(err)
	}

	return nil
}

// ExecTransaction esegue transaction in una transazione multi-documento (write concern majority).
// Un errore ritornato da transaction la abortisce; nil la committa.
//
// Passa da session.WithTransaction, cioè dal protocollo di retry del driver: una transazione
// abortita da un errore TransientTransactionError (conflitto di scrittura, elezione del primario) è
// rieseguita per intero, e un commit dall'esito ignoto (UnknownTransactionCommitResult, la rete
// caduta durante il commit) è ritentato — entro il limite di 120s del driver. Prima ogni errore del
// genere risaliva al chiamante, che nella prassi non ritentava: un conflitto fra due richieste
// concorrenti diventava un 500.
//
// Conseguenza da conoscere: transaction può essere eseguita PIÙ VOLTE. Dentro deve fare solo
// operazioni sul database col ctx che riceve (che porta la sessione); un effetto esterno — una
// chiamata HTTP, un messaggio pubblicato — sarebbe ripetuto a ogni tentativo.
func (s *Service) ExecTransaction(ctx context.Context, transaction func(ctx context.Context) error) *core.Error {
	session, err := s.Db().Client().StartSession()
	if err != nil {
		return errs.Tech(CodeTransaction).WithCause(err)
	}
	defer session.EndSession(ctx)

	txnOptions := options.Transaction().SetWriteConcern(writeconcern.Majority())
	if _, err := session.WithTransaction(ctx, func(sessCtx context.Context) (any, error) {
		return nil, transaction(sessCtx)
	}, txnOptions); err != nil {
		return errs.Tech(CodeTransaction).WithCause(err)
	}
	return nil
}

func (s *Service) GetIds(ctx context.Context, filter string, collectionName string, sort string, limit int) ([]string, *core.Error) {
	var filterMap map[string]any
	if err := json.Unmarshal([]byte(filter), &filterMap); err != nil {
		log.Error().Err(err).Msg("error unmarshal filter")
		return nil, errs.Tech(CodeProperties).WithMessage("error unmarshal filter").WithCause(err)
	}
	var sortMap map[string]int
	if sort != "" {
		if serr := json.Unmarshal([]byte(sort), &sortMap); serr != nil {
			log.Error().Err(serr).Msgf("error unmarshal sort: %s", serr.Error())
			return nil, errs.Tech(CodeProperties).WithMessage("error unmarshal sort").WithCause(serr)
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
		return nil, errs.Tech(CodeFind).WithCause(err)
	}
	defer mongoutil.CloseCursor(ctx, cursor, "GetIds")

	var ids []string
	for cursor.Next(ctx) {
		var result struct {
			Id string `bson:"_id"` // Campo _id come stringa
		}
		if errDecode := cursor.Decode(&result); errDecode != nil {
			return nil, errs.Tech(CodeCursor).WithCause(errDecode)
		}
		ids = append(ids, result.Id)
	}

	return ids, nil
}

func (s *Service) GetPageByFilter[T ICollection](ctx context.Context, filter IFilter, paging *page.Paging, opts ...options.Lister[options.FindOptions]) ([]T, *core.Error) {
	collection, collErr := s.collection(filter.GetFilterCollectionName(ctx), "")
	if collErr != nil {
		return nil, collErr
	}

	filterB, errB := buildFilter(filter)
	if errB != nil {
		return nil, errs.Tech(CodeFilter).WithCause(errB)
	}

	totalItems, errCount := collection.CountDocuments(ctx, filterB)
	if errCount != nil {
		return nil, errs.Tech(CodeCount).WithCause(errCount)
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
		return nil, errs.Tech(CodeFind).WithCause(errFind)
	}
	defer mongoutil.CloseCursor(ctx, cursor, "GetPageByFilter")

	var results []T
	if errDecode := cursor.All(ctx, &results); errDecode != nil {
		return nil, errs.Tech(CodeCursor).WithCause(errDecode)
	}

	return results, nil
}

func (s *Service) GetSequence(ctx context.Context, sequenceCollection, sequenceName string) (int, *core.Error) {
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
		return 0, errs.Tech(CodeSequence).WithCause(err)
	}

	if sequence, ok := result["sequence"].(int32); ok { // Assuming sequence is an int32
		return int(sequence), nil
	} else {
		return 0, errs.Tech(CodeSequenceInvalid).WithMessage("sequence is not an integer")
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

// buildWriteFilter è buildFilter per le scritture (UpdateOne/UpdateMany/ReplaceOne/DeleteOne/
// DeleteMany): un filtro vuoto è un errore, non `{}`. Un filtro coi campi tutti `omitempty` e tutti
// vuoti — tipicamente query param assenti — faceva aggiornare o cancellare l'intera collection
// (DeleteOne/UpdateOne/ReplaceOne: un documento qualsiasi). Chi vuole davvero toccare tutto lo
// scrive col driver, da Service.Db().
func buildWriteFilter(filter IFilter) (bson.M, *core.Error) {
	filterB, err := buildFilter(filter)
	if err != nil {
		return nil, errs.Tech(CodeFilter).WithCause(err)
	}
	if len(filterB) == 0 {
		return nil, errs.Business(CodeEmptyFilter).
			WithMessage("il filtro non esprime nessuna condizione: la scrittura toccherebbe tutti i documenti")
	}
	return filterB, nil
}
