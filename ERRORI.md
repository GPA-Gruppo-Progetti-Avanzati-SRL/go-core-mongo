# Codici di errore — go-core-mongo

Tutti i metodi del `*coremongo.Service` ritornano `*core.ApplicationError`. L'errore del driver
è **allegato come causa** (`WithCause`): un `mongo.ErrNoDocuments` resta raggiungibile con
`errors.Is` anche dentro un 404.

> **`Ambit` = `go-core-mongo`** (costante `coremongo.Ambit`) su **ogni** errore del modulo,
> `authorization` compreso: è il campo che dice da quale libreria viene il guasto. Senza,
> `ApplicationError.Ambit` resterebbe l'`AppName`, cioè l'app che l'errore lo riceve.
> I codici sono costanti esportate (`coremongo.CodeFilter`, …) in `errors.go`, e passano tutti
> dal costruttore `techErr(code)` / `notFound()`.

## Risorse non configurate

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `MONGO-COLL-NOTFOUND` | 500 | `CodeCollectionNotFound` | `collection.go:32` | `GetCollection` ha ritornato nil: collection assente da `mongo.collections`. È il guard che evita il panic in **tutti** i CRUD generici |
| `MONGO-COLL-NOTFOUND` | 500 | `authorization.codeCollectionNotFound` | `authorization/lut.go:106` | stessa condizione sulla collection `acl` |
| `MONGO-AGGR-NOTFOUND` | 500 | `CodeAggregationNotFound` | `aggregation.go:130,135,221` | aggregation assente dal registry di `WithAggregations`: rispettivamente `unionWith` senza argomento `pipeline`, `unionWith` verso un nome sconosciuto, `ExecuteAggregation` con nome sconosciuto |

## Lettura

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `MONGO-FILTER` | 500 | `CodeFilter` | `collection.go:65,84,106,131,197,219,241,263,289,396`, `aggregation.go:179` | `buildFilter` fallita: tag `field:`/`operator:` non validi. È l'errore che prima si confondeva con un guasto del driver |
| `MONGO-FINDONE` | 500 | `CodeFindOne` | `collection.go:54,95` | `FindOne` (o il decode del singolo documento) fallita |
| `MONGO-FIND` | 500 | `CodeFind` | `collection.go:370,417` | `Find` fallita (`GetIds`, `GetPageByFilter`) |
| `MONGO-CURSOR` | 500 | `CodeCursor` | `collection.go:380,423` | iterazione/decode del cursore fallita |
| `MONGO-COUNT` | 500 | `CodeCount` | `collection.go:73,401` | `CountDocuments` fallita |
| `MONGO-GOBF-ERRFIND` | 500 | `CodeGetObjectsFind` | `collection.go:114` | storico: `Find` in `GetObjectsByFilter` |
| `MONGO-GOBF-ERRCUR` | 500 | `CodeGetObjectsCursor` | `collection.go:120` | storico: cursore in `GetObjectsByFilter` |
| `MONGO-GOBFS-ERRFIND` | 500 | `CodeGetObjectsSorted` | `collection.go:140,146` | storico: `Find` **e** cursore nella variante sorted |
| `NOT-FOUND` | 404 | — | `collection.go:52,93`, `aggregation.go:241` | nessun documento: causa `mongo.ErrNoDocuments` allegata |

## Scrittura

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `MONGO-INSERT` | 500 | `CodeInsert` | `collection.go:161,183` | `InsertOne`/`InsertMany` fallita |
| `MONGO-INSERT-NOID` | 500 | `CodeInsertNoID` | `collection.go:164` | insert eseguita ma senza `InsertedID` |
| `INSERT-MISMATCH` | 500 | `CodeInsertMismatch` | `collection.go:188` | `InsertMany`: `InsertedIDs` diversi dagli elementi richiesti |
| `MONGO-UPDATE` | 500 | `CodeUpdate` | `collection.go:206,228` | `UpdateOne`/`UpdateMany` fallita |
| `MONGO-REPLACE` | 500 | `CodeReplace` | `collection.go:250` | `ReplaceOne` fallita |
| `MONGO-DELETE` | 500 | `CodeDelete` | `collection.go:272,298` | `DeleteOne`/`DeleteMany` fallita |
| `MON-AGGINC` | 500 | `CodeInconsistent` | `collection.go:210,232,254,279` | update/replace/delete che ha toccato **≠ 1** documento |
| `NOT-FOUND` | 404 | — | `collection.go:275` | delete con `DeletedCount == 0` |
| `MONGO-BULK` | 500 | `CodeBulk` | `bulk.go:90` | `BulkWrite` fallita. Il `*BulkResult` è nil: nessun contatore da leggere |
| `MONGO-TX` | 500 | `CodeTransaction` | `collection.go:310,333` | `StartSession` o `WithSession`/commit falliti |
| `MONGO-SEQ` | 500 | `CodeSequence` | `collection.go:449` | `FindOneAndUpdate` sulla collection di sequenze fallita |
| `SEQ-INV` | 500 | `CodeSequenceInvalid` | `collection.go:455` | il campo `sequence` non è un intero |

## Filtri, properties, aggregation

| Codice | HTTP | Costante | Origine | Significato |
|---|---|---|---|---|
| `PROPERTIES` | 500 | `CodeProperties` | `collection.go:342,348` | unmarshal di filtro / sort dalle properties fallito |
| `MON-FIL` | 500 | `CodeAggregationFilter` | `aggregation.go:173` | il filtro passato all'aggregation non implementa `IFilter` |
| `MON-SOR` | 500 | `CodeAggregationSort` | `aggregation.go:188,194,200,205` | sort dell'aggregation malformato: order non trovato / struttura assente / campo assente / verso assente |
| `MONGO-AGGR-OP` | 500 | `CodeAggregationOperator` | `aggregation.go:109` | stage con operatore non supportato dal builder |
| `MONGO-EXECAGGR` | 500 | `CodeExecAggregation` | `aggregation.go:243` | esecuzione della pipeline fallita |
| `MONGO-EXECAGGR-CUR` | 500 | `CodeExecAggregationCur` | `aggregation.go:253` | cursore dell'aggregation fallito |

## Autorizzazione (LUT ACL)

| Codice | HTTP | Origine | Significato |
|---|---|---|---|
| `MONGO-ACL-AGGR` | 500 | `authorization/lut.go:222` | aggregation di refresh della LUT fallita |
| `MONGO-ACL-CUR` | 500 | `authorization/lut.go:228` | lettura del cursore di refresh fallita |

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

`locker/` non definisce codici propri: ritorna `lock.ErrNotAcquired` (lease già tenuto, o
documento non ancora scaduto) e `lock.ErrLockLost` (rinnovo fallito) di `go-core-app/lock`.
