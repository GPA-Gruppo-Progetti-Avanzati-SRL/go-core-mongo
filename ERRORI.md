# Codici di errore — go-core-mongo

Tutti i metodi del `*coremongo.Service` ritornano `*core.ApplicationError`. Dove l'errore
viene dal driver, l'errore originale è **allegato come causa** (`WithCause`): un
`mongo.ErrNoDocuments` resta raggiungibile con `errors.Is` anche dentro un 404.

## Codici emessi

### Risoluzione della collection

| Codice | HTTP | Origine | Significato |
|---|---|---|---|
| `MONGO-COLL-NOTFOUND` | 500 | `collection.go:33` (`Service.collection`, costante `codeCollectionNotFound`) | `GetCollection` ha ritornato nil: la collection non è configurata in `mongo.collections`. È il guard che evita il panic su collection nil in **tutti** i CRUD generici |
| `MONGO-COLL-NOTFOUND` | 500 | `authorization/lut.go:106` | stessa condizione sulla collection `acl` durante il refresh della LUT di autorizzazione |

### Lettura

| Codice | HTTP | Origine | Significato |
|---|---|---|---|
| `NOT-FOUND` | 404 | `collection.go:53,94` | `FindOne` senza documenti: causa `mongo.ErrNoDocuments` allegata |
| `MONGO-GOBF-ERRFIND` | 500 | `collection.go:115` | `Find` fallita in `GetObjectsByFilter` |
| `MONGO-GOBF-ERRCUR` | 500 | `collection.go:121` | iterazione del cursore fallita in `GetObjectsByFilter` |
| `MONGO-GOBFS-ERRFIND` | 500 | `collection.go:141,147` | `Find` **e** iterazione cursore nella variante con sort/paginazione (stesso codice per le due condizioni) |

### Scrittura

| Codice | HTTP | Origine | Significato |
|---|---|---|---|
| `NOT-FOUND` | 404 | `collection.go:165` | `InsertOne` senza `InsertedID` |
| `INSERT-MISMATCH` | 500 | `collection.go:189` | `InsertMany`: numero di `InsertedIDs` diverso dagli elementi richiesti |
| `MON-AGGINC` | 500 | `collection.go:211,233,255` | **aggiornamento incoerente**: `ModifiedCount`/`MatchedCount` diverso da 1 |
| `NOT-FOUND` | 404 | `collection.go:276` | delete con `DeletedCount == 0` |
| `MON-AGGINC` | 500 | `collection.go:280` | **rimozione incoerente**: `DeletedCount` diverso da 1 |
| `MONGO-BULK` | 500 | `bulk.go:90` | `BulkWrite` fallita. Il `*BulkResult` è nil: nessun contatore da leggere |
| `SEQ-INV` | 500 | `collection.go:457` | il campo `sequence` del documento di sequenza non è un intero |
| `TECH500` | 500 | `collection.go:162,184,274`, … | errore del driver senza codice specifico (default di `TechnicalError()`); l'errore mongo è la causa |

### Filtri e properties

| Codice | HTTP | Origine | Significato |
|---|---|---|---|
| `PROPERTIES` | 500 | `collection.go:343` | unmarshal del filtro dalle properties fallito |
| `PROPERTIES` | 500 | `collection.go:349` | unmarshal del sort dalle properties fallito |
| `MON-FIL` | 500 | `aggregation.go:171` | il filtro passato all'aggregation non implementa `IFilter` |

### Aggregation

| Codice | HTTP | Origine | Significato |
|---|---|---|---|
| `MONGO-AGGR-NOTFOUND` | 500 | `aggregation.go:226` | aggregation non presente nel registry costruito da `WithAggregations`: è una **misconfigurazione**, stessa natura di `MONGO-COLL-NOTFOUND` |
| `NOT-FOUND` | 404 | `aggregation.go:238` | `Aggregate` con `mongo.ErrNoDocuments` |
| `MONGO-EXECAGGR` | 500 | `aggregation.go:240` | esecuzione della pipeline fallita |
| `MONGO-EXECAGGR-CUR` | 500 | `aggregation.go:250` | iterazione del cursore dell'aggregation fallita |
| `UNKNOWN Operator` | 500 | `aggregation.go:109` | stage con operatore non supportato dal builder |
| `MON-SOR` | 500 | `aggregation.go:186,192,198,203` | sort dell'aggregation malformato: order non trovato / struttura assente / campo assente / verso assente |
| `MONGO-AGGR-NOTFOUND` | 500 | `aggregation.go:135` | `unionWith` senza l'argomento `pipeline` (assente o non stringa) |
| `MONGO-AGGR-NOTFOUND` | 500 | `aggregation.go:140` | `unionWith` che referenzia un'aggregation non configurata |

## Convenzioni

- `MON-*` = errori semantici della libreria (incoerenza, sort, filtro).
- `MONGO-*` = errori del driver Mongo (con l'errore originale in causa) e risorse non
  configurate: `MONGO-COLL-NOTFOUND`, `MONGO-AGGR-NOTFOUND`.
- I `NOT-FOUND` conservano sempre `mongo.ErrNoDocuments` come causa: un chiamante può
  distinguere il "nessun documento" del driver da un 404 sintetico.

## Errori sentinella

`locker/` non definisce codici propri: ritorna `lock.ErrNotAcquired` (lease già tenuto, o
documento non scaduto) e `lock.ErrLockLost` (rinnovo fallito) di `go-core-app/lock`.
