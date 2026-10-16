# Codici di errore — go-core-mongo

Tutti i metodi del `*coremongo.Service` ritornano `*core.Error`. L'errore del driver
è **allegato come causa** (`WithCause`): un `mongo.ErrNoDocuments` resta raggiungibile con
`errors.Is` anche dentro un 404.

> **`Ambit` = `go-core-mongo`** (costante `coremongo.Ambit`) su **ogni** errore del modulo:
> è il campo che dice da quale libreria viene il guasto. Senza,
> `core.Error.Ambit` resterebbe l'`AppName`, cioè l'app che l'errore lo riceve.
> I codici sono costanti esportate (`coremongo.CodeFilter`, …) in `errors.go`, e passano tutti
> da `errs.Tech(code)` / `errs.NotFound()` (`errs = core.AmbitErrors{Ambit: Ambit}`).

## Risorse non configurate

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `MONGO-COLL-NOTFOUND` | 500 | `CodeCollectionNotFound` | `collection.go` | `GetCollection` ha ritornato nil: collection assente da `mongo.collections`. È il guard che evita il panic in **tutti** i CRUD generici |
| `MONGO-AGGR-NOTFOUND` | 500 | `CodeAggregationNotFound` | `aggregation.go` | aggregation assente dal registry di `WithAggregations`: rispettivamente `unionWith` senza argomento `pipeline`, `unionWith` verso un nome sconosciuto, `ExecuteAggregation` con nome sconosciuto |

## Lettura

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `MONGO-FILTER` | 500 | `CodeFilter` | `collection.go`, `aggregation.go` | `buildFilter` fallita: tag `field:`/`operator:` non validi. È l'errore che prima si confondeva con un guasto del driver |
| `MONGO-SORT` | 422 | `CodeSort` | `collection.go` (`GetObjectsByFilterSorted`) | campo di sort che non è un identificatore (`page.ValidSortField`): in bson una chiave `$...` sarebbe un operatore |
| `MONGO-EMPTY-FILTER` | 422 | `CodeEmptyFilter` | `collection.go` (`buildWriteFilter`: `UpdateOne`, `UpdateMany`, `ReplaceOne`, `DeleteOne`, `DeleteMany`) | il filtro di una scrittura è vuoto (campi `omitempty` tutti vuoti): toccherebbe tutti i documenti, quindi l'operazione non parte |
| `MONGO-FINDONE` | 500 | `CodeFindOne` | `collection.go` | `FindOne` (o il decode del singolo documento) fallita |
| `MONGO-FIND` | 500 | `CodeFind` | `collection.go` | `Find` fallita (`GetIds`, `GetPageByFilter`) |
| `MONGO-CURSOR` | 500 | `CodeCursor` | `collection.go` | iterazione/decode del cursore fallita |
| `MONGO-COUNT` | 500 | `CodeCount` | `collection.go` | `CountDocuments` fallita |
| `MONGO-GOBF-ERRFIND` | 500 | `CodeGetObjectsFind` | `collection.go` | storico: `Find` in `GetObjectsByFilter` |
| `MONGO-GOBF-ERRCUR` | 500 | `CodeGetObjectsCursor` | `collection.go` | storico: cursore in `GetObjectsByFilter` |
| `MONGO-GOBFS-ERRFIND` | 500 | `CodeGetObjectsSorted` | `collection.go` | storico: `Find` **e** cursore nella variante sorted |
| `NOT-FOUND` | 404 | — | `collection.go`, `aggregation.go` | nessun documento: causa `mongo.ErrNoDocuments` allegata |

## Scrittura

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `MONGO-INSERT` | 500 | `CodeInsert` | `collection.go` | `InsertOne`/`InsertMany` fallita |
| `MONGO-INSERT-NOID` | 500 | `CodeInsertNoID` | `collection.go` | insert eseguita ma senza `InsertedID` |
| `INSERT-MISMATCH` | 500 | `CodeInsertMismatch` | `collection.go` | `InsertMany`: `InsertedIDs` diversi dagli elementi richiesti |
| `MONGO-UPDATE` | 500 | `CodeUpdate` | `collection.go` | `UpdateOne`/`UpdateMany` fallita |
| `MONGO-REPLACE` | 500 | `CodeReplace` | `collection.go` | `ReplaceOne` fallita |
| `MONGO-DELETE` | 500 | `CodeDelete` | `collection.go` | `DeleteOne`/`DeleteMany` fallita |
| `MON-AGGINC` | 500 | `CodeInconsistent` | `collection.go` (`UpdateMany`, `DeleteOne`) | `UpdateMany` che ha **trovato** un numero di documenti diverso dall'atteso; `DeleteOne` che ne ha cancellati più di uno |
| `NOT-FOUND` | 404 | — | `collection.go` (`UpdateOne`, `ReplaceOne`, `DeleteOne`) | nessun documento trovato (e nessun upsert). Fino al 2026-10-01 `UpdateOne`/`ReplaceOne` controllavano `ModifiedCount` e rispondevano `MON-AGGINC` anche a un update che riscriveva gli stessi valori |
| `MONGO-BULK` | 500 | `CodeBulk` | `bulk.go` | `BulkWrite` fallita. Il `*BulkResult` è nil: nessun contatore da leggere |
| `MONGO-TX` | 500 | `CodeTransaction` | `collection.go` | `StartSession` o `WithSession`/commit falliti |
| `MONGO-SEQ` | 500 | `CodeSequence` | `collection.go` | `FindOneAndUpdate` sulla collection di sequenze fallita |
| `SEQ-INV` | 500 | `CodeSequenceInvalid` | `collection.go` | il campo `sequence` non è un intero |

## Filtri, properties, aggregation

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `PROPERTIES` | 500 | `CodeProperties` | `collection.go` | unmarshal di filtro / sort dalle properties fallito |
| `MON-FIL` | 500 | `CodeAggregationFilter` | `aggregation.go` | il filtro passato all'aggregation non implementa `IFilter` |
| `MON-SOR` | 500 | `CodeAggregationSort` | `aggregation.go` | sort dell'aggregation malformato: order non trovato / struttura assente / campo assente / verso assente |
| `MONGO-AGGR-OP` | 500 | `CodeAggregationOperator` | `aggregation.go` | stage con operatore non supportato dal builder |
| `MONGO-EXECAGGR` | 500 | `CodeExecAggregation` | `aggregation.go` | esecuzione della pipeline fallita |
| `MONGO-EXECAGGR-CUR` | 500 | `CodeExecAggregationCur` | `aggregation.go` | cursore dell'aggregation fallito |

L'ACL non è più di questo modulo: i suoi errori sono quelli di `go-core-auth/mongosource` (`AUTH-MONGO-*`, vedi `go-core-auth/ERRORI.md`).

## Cambiamenti rispetto al censimento precedente

- **~36 siti ricadevano su `TECH500`**, il default: un errore di costruzione del filtro, un
  `Find` fallito e un commit di transazione fallito erano indistinguibili nel log. Ora ogni
  operazione ha il suo codice.
- **`InsertOne` senza `InsertedID` non è più un 404**: era `NOT-FOUND`, cioè un guasto tecnico
  travestito da "oggetto non trovato" → `MONGO-INSERT-NOID` (500).
- **`UNKNOWN Operator` conteneva uno spazio**, quindi non era usabile come codice →
  `MONGO-AGGR-OP`.
- **`GetIds` spezzava la catena degli errori**: avvolgeva l'errore del driver con
  `fmt.Errorf("error Mongo: %s", ...)` (`%s`, non `%w`), quindi `errors.Is` non arrivava più
  all'errore mongo. Ora la causa è l'errore originale.
- I codici storici (`MON-AGGINC`, `MON-FIL`, `MON-SOR`, `SEQ-INV`, `PROPERTIES`,
  `INSERT-MISMATCH`, `MONGO-GOBF*`) sono **mantenuti**: rinominarli avrebbe rotto le app senza
  aggiungere informazione.

**Convenzioni:** `MONGO-*` = operazione del driver o risorsa non configurata (errore originale
in causa); `MON-*` = incoerenza semantica rilevata dalla libreria; i `NOT-FOUND` conservano
sempre `mongo.ErrNoDocuments` come causa, così un 404 del driver resta distinguibile da un 404
sintetico.

## Errori sentinella

Il lock distribuito non è più di questo modulo: `ErrNotAcquired` ed `ErrLockLost` sono di `go-core-locker` (`corelock`).
