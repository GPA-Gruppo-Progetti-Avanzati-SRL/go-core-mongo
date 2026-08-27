package coremongo

import (
	"context"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// BulkResult riporta l'esito aggregato di una BulkWrite traducendo i contatori del driver. È
// aggregato per batch, non per operazione: se il batch mescola tipi di WriteModel diversi (upsert e
// cancellazioni logiche, tipico di un sink CDC) i loro esiti non sono più distinguibili qui, eccetto
// Inserted/Upserted/Deleted che solo un tipo di operazione può produrre.
//   - Inserted: documenti creati da un InsertOneModel.
//   - Upserted: documenti creati da un Update/Replace con SetUpsert(true) che non ha trovato il filtro.
//   - Modified: documenti esistenti effettivamente modificati.
//   - Unchanged: documenti trovati dal filtro ma non modificati (MatchedCount - ModifiedCount) — il
//     contenuto era già identico, oppure la update pipeline ha rimesso `$$ROOT` perché una guardia
//     applicativa non era soddisfatta (evento più vecchio di quello persistito). Non conta le
//     operazioni che non hanno trovato nulla: quelle non emergono in nessun contatore.
//   - Deleted: documenti rimossi da un DeleteOne/DeleteManyModel (cancellazione fisica; una
//     cancellazione logica è un update, quindi conta in Modified).
type BulkResult struct {
	Inserted  int64
	Upserted  int64
	Modified  int64
	Unchanged int64
	Deleted   int64
}

// BulkOrdered forza l'esecuzione sequenziale del batch: necessaria quando lo stesso `_id` può comparire
// più di una volta con operazioni diverse mescolate (un update e una cancellazione logica, tipico di un
// sink CDC) — con unordered MongoDB non garantisce l'ordine reale d'arrivo, e due scritture concorrenti
// sullo stesso `_id` con upsert possono anche generare un duplicate key error. È il default del driver,
// ma passarla esplicitamente dichiara che l'ordine è una precondizione di correttezza e non un default
// ereditato per caso.
func BulkOrdered() options.Lister[options.BulkWriteOptions] {
	return options.BulkWrite().SetOrdered(true)
}

// BulkUnordered lascia MongoDB libero di eseguire il batch senza garanzie d'ordine, più efficiente lato
// server. Sicura SOLO se il chiamante garantisce a monte al più una scrittura per `_id` nel batch
// (tipicamente perché il batch è già stato compattato per chiave a monte): con quella garanzia non c'è
// più nessun ordine relativo da preservare, quindi nulla da perdere lasciando decidere al server.
func BulkUnordered() options.Lister[options.BulkWriteOptions] {
	return options.BulkWrite().SetOrdered(false)
}

// BulkOrderedIf sceglie fra le due secondo la garanzia del chiamante: compacted=true significa "al più
// una scrittura per `_id` in questo batch" -> BulkUnordered, altrimenti BulkOrdered. Esiste perché la
// scelta è sempre questa domanda, e scriverla come un if sul sito di chiamata invita a invertirla.
func BulkOrderedIf(compacted bool) options.Lister[options.BulkWriteOptions] {
	if compacted {
		return BulkUnordered()
	}
	return BulkOrdered()
}

// BulkWrite esegue models in un'unica BulkWrite e traduce il risultato in BulkResult. Come gli altri
// CRUD generici la destinazione arriva dal tipo scritto (T.GetCollectionName), non da un parametro: un
// batch di WriteModel è opaco — il driver non permette di risalire dai modelli all'entità — quindi T è
// anche l'unica dichiarazione, verificata dal compilatore, di che cosa quei modelli stiano scrivendo.
// T non è inferibile dagli argomenti e va istanziato esplicitamente (s.BulkWrite[MioDocumento](...)),
// e deve essere il tipo valore, non un puntatore: il nome della collezione è letto da uno zero value,
// come in GetObjectById/GetObjectByFilter.
//
// L'ordine di esecuzione NON ha un default: è opts a deciderlo (BulkOrdered/BulkUnordered/BulkOrderedIf),
// perché la sicurezza di unordered dipende dalla forma dei WriteModel passati — è lecita solo se il
// chiamante garantisce al più una scrittura per `_id` nel batch, altrimenti l'ordine relativo di due
// operazioni sullo stesso documento va perso (e due scritture concorrenti sullo stesso `_id` con
// upsert possono anche produrre un duplicate key error). Il default del driver è ordered.
//
// Il risultato è nil se e solo se non è stata eseguita alcuna BulkWrite: con models vuota — un batch
// senza nulla da scrivere è un no-op, non un errore — oppure insieme a un errore. Un nil è quindi
// distinguibile da un batch eseguito i cui contatori sono tutti a zero (tutte operazioni a vuoto), e
// va gestito dal chiamante prima di leggere i contatori.
func (s *Service) BulkWrite[T ICollection](ctx context.Context, models []mongo.WriteModel, opts ...options.Lister[options.BulkWriteOptions]) (*BulkResult, *core.ApplicationError) {
	if len(models) == 0 {
		return nil, nil
	}
	var obj T
	collection := obj.GetCollectionName(ctx)
	coll, collErr := s.collection(collection, "")
	if collErr != nil {
		return nil, collErr
	}
	res, err := coll.BulkWrite(ctx, models, opts...)
	if err != nil {
		return nil, core.TechnicalError().WithCode("MONGO-BULK").WithCause(err)
	}
	return &BulkResult{
		Inserted:  res.InsertedCount,
		Upserted:  res.UpsertedCount,
		Modified:  res.ModifiedCount,
		Unchanged: res.MatchedCount - res.ModifiedCount,
		Deleted:   res.DeletedCount,
	}, nil
}
