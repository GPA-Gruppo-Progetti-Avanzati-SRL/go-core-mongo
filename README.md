# GO-CORE-MONGO

## Installation

    go get github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo

---

La libreria ```go-core-mongo``` è una libreria per il linguaggio di programmazione Go che fornisce funzionalità per interagire con un database MongoDB. Essa include strumenti e metodi per connettersi a MongoDB, gestire connessioni e configurazioni, e altre operazioni comuni necessarie per lavorare con MongoDB in applicazioni Go.

Di default permette all'applicazione di esporre le metriche del database.

## Wiring

`coremongo.Module` è l'unico entry-point: supplisce la `Config` e fornisce
`*coremongo.Service`, l'unico handle Mongo dell'applicazione (connessione, CRUD
generici, aggregation). Lo consumano direttamente anche `locker`, `authorization` e
`go-core-batch/store/mongostore`: la `mongolks.LinkedService` resta un dettaglio
interno e non va iniettata in giro.

```go
package services

import coremongo "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo"

type Config struct {
    Mongo coremongo.Config `yaml:"mongo" mapstructure:"mongo" json:"mongo"`
    // ...
}

func ProvideServices(cfg *Config) {
    coremongo.Module(&cfg.Mongo)
}
```

`coremongo.Config` è un alias di `mongolks.Config`: l'app non importa
`tpm-mongo-common`. Il costruttore non è esportato, quindi non serve (né si può fare)
`core.Supply`/`core.Provide` a mano.

| Opzione | Effetto |
|---------|---------|
| `WithModes(modes...)` | registra solo quando `core.Mode` è tra i modes indicati; senza opzione registra sempre |
| `WithAggregations(dir)` | carica le pipeline di aggregation dalla FS indicata (vedi sotto) |
| `WithAuthorization()` | fornisce l'`authorization.Authorizer` di go-core-app alimentato dalla collection ACL |

`WithAuthorization()` sostituisce il `core.ProvideAs[authorization.Authorizer](mongoauth.NewAuthorizationLut)`
che l'app faceva a mano in `main.go`: così l'app non importa `go-core-app/authorization` solo per
nominare un tipo, e soprattutto la LUT **eredita i modes del Module** — a mano era facile scordarselo
e ritrovarsi la LUT che interroga Mongo anche in un processo worker.

```go
coremongo.Module(&cfg.Mongo,
    coremongo.WithAggregations(data.AggregationFiles),
    coremongo.WithAuthorization(),
    coremongo.WithModes(engine.Api))
```

```yaml
config:
  services:
    mongo:
      name: default
      host: "mongodb+srv://<host>/?tls=false"
      db-name: mydb
      user: ${MONGODB_USR_ENV}
      pwd:  ${MONGODB_PWD_ENV}
      collections:
        - id: acl          # obbligatoria per l'autorizzazione
          name: acl
        - id: people       # una entry per ogni collection usata
          name: people
```

Ogni collection usata dall'app va dichiarata in `collections`: `GetCollection` risolve
per `id` e ritorna `nil` per un id non dichiarato.

## Funzionalità principali

### Filter Builder

Il Filter Builder è una funzionalità della libreria go-core-mongo che permette di costruire filtri per le query MongoDB a partire da una struct Go. Questa funzionalità converte una struct con tag specifici in un oggetto bson.M, che può essere utilizzato nelle query MongoDB.

La struct di input deve avere i campi taggati con:

- **field**: "nome_campo_mongodb": Il nome del campo in MongoDB.
- **operator**: "$operatore": L'operatore MongoDB da usare
- **omitempty**: `"true"`: salta il campo se è a zero-value

**Operatori supportati:**

| Famiglia | Operatori |
|---|---|
| confronto | `$eq` `$ne` `$lt` `$lte` `$gt` `$gte` |
| liste | `$in` `$nin` `$all` |
| presenza | `$exists` |
| stringhe | `$startswith` `$istartswith` `$endswith` `$iendswith` `$contains` `$icontains` `$regex` |
| array | `$size` |

Gli operatori sulle stringhe sono tradotti in `$regex` con l'ancoraggio giusto; le varianti con la
`i` iniziale sono case-insensitive.

```go
Filtro struct {
    Nome  string `field:"name" operator:"$eq"`
    Eta   int    `field:"age" operator:"$gt"`
    Tags  []string `field:"tags" operator:"$in"`
}
```

Il Filter Builder itera attraverso i campi della struct, legge i tag field e operator, e costruisce un filtro **bson.M** che può essere utilizzato nelle query MongoDB.

**N.B.** per il momento è supportato solo **bson.M**

```go
filterStruct := Filtro{
    Nome: "Federico",
    Età:  28,
    Tags: []string{"developer", "backend", "frontend"},
}

filter, err := buildFilter(filterStruct)
if err != nil {
    log.Fatal(err)
}

// Il filtro risultante sarà:
// bson.M{
//     "name": bson.M{"$eq": "Federico"},
//     "age":  bson.M{"$gt": 28},
//     "tags": bson.M{"$in": []string{"developer", "backend", "frontend"}},
// }
```


### CRUD generici — metodi del Service

I CRUD sono **metodi generici di `*coremongo.Service`** (Go 1.27+): il Service è l'unico handle
Mongo dell'applicazione e viene iniettato da fx. `T` implementa `ICollection` — è da lì che arriva
la collection.

```go
// Lettura
item,  appErr := s.GetObjectById[T](ctx, id)
item,  appErr := s.GetObjectByFilter[T](ctx, filter)
items, appErr := s.GetObjectsByFilter[T](ctx, filter)
items, appErr := s.GetObjectsByFilterSorted[T](ctx, filter, sort)
items, appErr := s.GetPageByFilter[T](ctx, filter, paging, opts...)
n,     appErr := s.CountDocuments(ctx, filter)

// Scrittura
id,   appErr := s.InsertOne[T](ctx, obj, opts...)
appErr := s.InsertMany[T](ctx, list, opts...)
appErr := s.UpdateOne(ctx, filter, update, opts...)
appErr := s.UpdateMany(ctx, filter, update, n)
appErr := s.ReplaceOne[T](ctx, filter, obj, opts...)
appErr := s.DeleteOne(ctx, filter, opts...)
appErr := s.DeleteMany(ctx, filter, opts...)

// Transazioni e sequenze
appErr := s.ExecTransaction(ctx, func(ctx context.Context) error { ... })
seq,  appErr := s.GetSequence(ctx, "sequences", "person-id")
```

I `NotFoundError` conservano `mongo.ErrNoDocuments` come causa: è recuperabile con `errors.Is`
senza parsare il messaggio.

> **Nota sul linguaggio:** un metodo generico non può implementare un metodo di interfaccia, quindi
> `*Service` non è assegnabile a un'interfaccia che dichiari questi CRUD. Il data layer dell'app
> espone i propri metodi concreti (`IData`) e richiama al loro interno quelli del Service.

### Scritture in batch (BulkWrite)

`Service.BulkWrite[T]` esegue una lista di `mongo.WriteModel` in **una sola** BulkWrite. La collection
arriva dal type param (`T` implementa `ICollection`), non da un parametro: un batch di `WriteModel` è
opaco — dal modello non si risale all'entità — quindi `T` è anche l'unica dichiarazione, verificata dal
compilatore, di che cosa quei modelli stiano scrivendo. `T` non è inferibile dagli argomenti, va
istanziato esplicitamente, e deve essere il tipo **valore** (il nome è letto da uno zero value, come in
`GetObjectById`).

```go
models := make([]mongo.WriteModel, 0, len(eventi))
for _, e := range eventi {
    models = append(models, mongo.NewReplaceOneModel().
        SetFilter(bson.M{"_id": e.ID}).
        SetReplacement(e).
        SetUpsert(true))
}

res, appErr := svc.BulkWrite[model.Evento](ctx, models, coremongo.BulkUnordered())
if appErr != nil {
    return appErr
}
if res == nil {
    // nessuna BulkWrite eseguita: il batch era vuoto
    return nil
}
log.Debug().Int64("upserted", res.Upserted).Int64("modified", res.Modified).
    Int64("unchanged", res.Unchanged).Msg("batch scritto")
```

- **Il risultato è un `*BulkResult`, `nil` se e solo se nessuna BulkWrite è stata eseguita** (batch
  vuoto, oppure insieme a un errore): un nil è quindi distinguibile da un batch eseguito con tutti i
  contatori a zero, e va gestito prima di leggerli. I contatori sono la traduzione di quelli del
  driver: `Inserted` (da un `InsertOneModel`), `Upserted` (creati da un upsert), `Modified`,
  `Unchanged` (`MatchedCount - ModifiedCount`: trovati ma non modificati — contenuto già identico, o
  update pipeline che ha rimesso `$$ROOT` perché una guardia applicativa non era soddisfatta) e
  `Deleted`. Sono **aggregati per batch**: se il batch mescola tipi di operazione diversi, i loro esiti
  non sono più distinguibili qui.
- **L'ordine di esecuzione non ha un default implicito**: lo passa il chiamante con `BulkOrdered()`,
  `BulkUnordered()` o `BulkOrderedIf(compacted)`. `BulkUnordered` è più efficiente lato server ma è
  sicura SOLO se il chiamante garantisce al più una scrittura per `_id` nel batch (tipicamente perché
  l'ha compattato a monte per chiave); altrimenti l'ordine relativo di due operazioni sullo stesso
  documento va perso, e due upsert concorrenti sullo stesso `_id` inesistente possono anche produrre un
  duplicate key error. `BulkOrderedIf` è la stessa domanda scritta una volta sola.

### Aggregation Pipeline generator

Le pipeline sono file YAML embeddati nel binario, **uno per pipeline**. Non stanno nel
file di configurazione: il loro percorso è un fatto di compile-time, non di ambiente.

#### Registrazione

```go
//go:embed aggregations
var aggregationFiles embed.FS

coremongo.Module(&cfg.Mongo, coremongo.WithAggregations(aggregationFiles))
```

Si passa la variabile del `//go:embed` così com'è (`embed.FS` implementa `fs.FS`): la
libreria percorre la FS in ricorsione e carica ogni file `.yaml`/`.yml` a qualsiasi
profondità, ignorando gli altri. Il nome della cartella non compare da nessuna parte.

Ogni pipeline è indicizzata per il suo campo `name` — il nome del file non conta — in un
registry che appartiene al `Service`: due Service non si sovrascrivono le pipeline.

Il caricamento è fail-fast: FS senza file YAML, YAML non parsabile, `name` mancante o
`name` duplicato fermano l'avvio dell'applicazione invece di emergere alla prima query.

#### Struttura delle aggregazioni

Ogni file descrive una `Aggregation`:

- **name**: il nome con cui la pipeline viene eseguita (chiave del registry).
- **collection**: la collection MongoDB di partenza.
- **stages**: la lista di `Stage` che compone la pipeline.

Ogni `Stage`:

- **operator**: l'operatore MongoDB. Supportati: `$match`, `$project`, `$group`,
  `$addFields`, `$sort`, `$skip`, `$limit`, `$unionWith`.
- **args**: argomenti statici, scritti nel file.
- **key**: chiave con cui lo stage pesca il proprio valore dai `params` passati a
  runtime. `key` e `args` sono alternativi.

```yaml
name: example
collection: example
stages:
  - operator: $match
    key: match
  - operator: $skip
    key: skip
  - operator: $limit
    key: limit
```

`$match` con `key` vuole un `IFilter` nei params, che viene passato al filter builder;
senza params usa `args` così com'è. `$sort` prende l'ordinamento da `args.order`:

```yaml
  - operator: $sort
    args:
      order:
        - field: createTime
          verse: desc
```

`$unionWith` compone per nome un'altra pipeline dello stesso registry:

```yaml
  - operator: $unionWith
    args:
      pipeline: altra-pipeline
```

#### Esecuzione

`ExecuteAggregation` è un metodo generico del `Service` e decodifica i risultati in
`[]*T`:

```go
items, err := d.Service.ExecuteAggregation[models.Sottoscrizione](ctx, "example",
    map[string]any{
        "match": MyFilter{Field: "value"},
        "skip":  0,
        "limit": 50,
    },
    options.Aggregate().SetAllowDiskUse(true), // opts variadic, opzionali
)
if err != nil {
    return nil, err
}
```

Errori: `NOT-FOUND` (business) se il nome non è nel registry; `MONGO-EXECAGGR` e
`MONGO-EXECAGGR-CUR` (tecnici) su errore del driver o durante la lettura del cursore.

---

## Errori

Catalogo dei codici in **[ERRORI.md](ERRORI.md)**. Ogni errore che nasce dentro la libreria porta
`Ambit = coremongo.Ambit` (`"go-core-mongo"`) e un `Code` (`coremongo.CodeFindOne`, `CodeFilter`,
`CodeBulk`, …) invece di presentarsi come un errore dell'applicazione — i costruttori base di `core`
riempiono l'ambit con l'`AppName`, cioè con chi *riceve* l'errore. La causa reale resta raggiungibile
con `errors.Is`/`errors.As`: `mongo.ErrNoDocuments` è conservato anche dentro un `NotFoundError`.

## Lock distribuito — `locker`

`locker` implementa il [`lock.Locker`](../go-core-app) neutro di go-core-app su MongoDB: documenti
di lease con TTL in una collection dedicata (`scheduler_locks`), mutua esclusione via upsert atomico.
Non serve altra infrastruttura oltre alla connessione Mongo che l'app già usa, e non c'è nessuna
dipendenza da gocron.

```go
import mongolocker "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo/locker"

coremongo.Module(&cfg.Mongo)

batch.Module(&cfg.Batch, Register,
    batch.WithStore(storemongo.Module),
    batch.WithLocker(mongolocker.Module),   // mongo-only → niente Redis da deployare
    // ...
)
```

`locker.Module(modes ...string)` è **modes-only**: consuma il `*coremongo.Service` e registra
`lock.Locker`. La collection non va dichiarata in `collections:` — il locker usa il database grezzo.

Senza opzioni `Acquire` fa un solo tentativo non bloccante con TTL di **30s** e ritorna
`lock.ErrNotAcquired` in contesa (semantica dispatch-dedup); `lock.WithTries`/`WithRetryDelay`/
`WithExpiry` e `Handle.Extend` coprono la mutua esclusione di una sezione critica lunga.

---

## Comandi

```bash
go build ./...
go test ./...
go test -race -count=2 ./...
go vet ./...
```
