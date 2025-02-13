# GO-CORE-MONGO

## Installation

    go get github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo

---

La libreria ```go-core-mongo``` è una libreria per il linguaggio di programmazione Go che fornisce funzionalità per interagire con un database MongoDB. Essa include strumenti e metodi per connettersi a MongoDB, gestire connessioni e configurazioni, e altre operazioni comuni necessarie per lavorare con MongoDB in applicazioni Go.

Di default permette all'applicazione di esporre le metriche del database.

## Funzionalità principali

### Filter Builder

Il Filter Builder è una funzionalità della libreria go-core-mongo che permette di costruire filtri per le query MongoDB a partire da una struct Go. Questa funzionalità converte una struct con tag specifici in un oggetto bson.M, che può essere utilizzato nelle query MongoDB.

La struct di input deve avere i campi taggati con:

- **field**: "nome_campo_mongodb": Il nome del campo in MongoDB.
- **operator**: "$operatore": L'operatore MongoDB da usare
  - **operatori supportati**:
    - ```$eq```
    - ```$ne```
    - ```$lt```
    - ```$lte```
    - ```$gt```
    - ```$gte```
    - ```$in```
    - ```$nin```
    - ```$exists```

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

### Aggregation Pipeline generator

le pipeline venegono specificate nel file di configurazione che utilizza l'app che importa la ```go-core-mongo```

#### Struttura delle Aggregazioni

Le aggregazioni sono definite tramite la struct Aggregation, che include:

- **Name**: Il nome dell'aggregazione.
- **Collection**: La collezione MongoDB su cui eseguire l'aggregazione.
- **Stages**: Una lista di fasi (Stage) che compongono la pipeline di aggregazione.

Ogni Stage include:

- **Key**: Una chiave per identificare i parametri della fase.
- **Operator**: L'operatore MongoDB da utilizzare (es. ```$match```, ```$project```).
- **Args**: Argomenti specifici per l'operatore.

#### Esecuzioni dell'aggregazione

La funzione ```ExecuteAggregation``` esegue una pipeline di aggregazione su una collezione MongoDB. Questa funzione prende il nome di un'aggregazione predefinita, i parametri per la pipeline e le opzioni di aggregazione, e restituisce un cursore MongoDB con i risultati dell'aggregazione.

*Parametri*:

- **ctx**: Il contesto per l'esecuzione della query.
- **name**: Il nome dell'aggregazione predefinita.
- **params**: Una mappa di parametri da utilizzare nella pipeline di aggregazione.
- **opts**: Opzioni aggiuntive per l'aggregazione.

*Ritorna*:

- ```mongo.Cursor```: Un cursore MongoDB con i risultati dell'aggregazione.
- ```core.ApplicationError```: Un errore applicativo in caso di fallimento.

Esempio:

```go
ctx := context.TODO()
name := "exampleAggregation"
params := map[string]any{
    "stage1": MyFilter{Field: "value"},
}
opts := options.Aggregate()

cursor, err := service.ExecuteAggregation(ctx, name, params, opts)
if err != nil {
    log.Fatal(err)
}

for cursor.Next(ctx) {
    var result bson.M
    if err := cursor.Decode(&result); err != nil {
        log.Fatal(err)
    }
    fmt.Println(result)
}
```

*Dettagli di implementazione*

1. La funzione cerca l'aggregazione predefinita nel mappa Aggregations utilizzando il nome fornito.
2. Se l'aggregazione non viene trovata, restituisce un errore di tipo BusinessError.
3. Genera la pipeline di aggregazione chiamando la funzione GenerateAggregation con l'aggregazione e i parametri forniti.
4. Converte la pipeline in formato JSON per il logging.
5. Esegue l'aggregazione sulla collezione specificata utilizzando il metodo Aggregate di MongoDB.
6. Gestisce eventuali errori, inclusi i casi in cui non vengono trovati documenti (mongo.ErrNoDocuments).
7. Restituisce il cursore con i risultati dell'aggregazione.

Questa funzione permette di eseguire aggregazioni complesse in modo dinamico, basandosi su configurazioni predefinite e parametri forniti a runtime.

#### Configurazione delle aggregazioni

Le aggregazioni vengono definite e configurate tramite file di configurazione (es. JSON, YAML) e caricate nell'applicazione. Questo permette di definire e modificare le pipeline di aggregazione senza dover cambiare il codice sorgente.

Esempio di configurazione YAML:

```yaml
aggregations:
  - name: exampleAggregation
    collection: exampleCollection
    stages:
      - operator: $match
        key: stage1
      - operator: $project
        args:
          field: 1
      - operator: $skip
        key: skip
      - operator: $limit
        key: limit
```
