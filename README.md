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
MyFilter struct {
    Name  string `field:"name" operator:"$eq"`
    Age   int    `field:"age" operator:"$gt"`
    Tags  []string `field:"tags" operator:"$in"`
}
```

Il Filter Builder itera attraverso i campi della struct, legge i tag field e operator, e costruisce un filtro **bson.M** che può essere utilizzato nelle query MongoDB.

**N.B.** per il momento è supportato solo **bson.M**

```go
filterStruct := MyFilter{
    Name: "John",
    Age:  30,
    Tags: []string{"developer", "gopher"},
}

filter, err := buildFilter(filterStruct)
if err != nil {
    log.Fatal(err)
}

// Il filtro risultante sarà:
// bson.M{
//     "name": bson.M{"$eq": "John"},
//     "age":  bson.M{"$gt": 30},
//     "tags": bson.M{"$in": []string{"developer", "gopher"}},
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

- Key: Una chiave per identificare i parametri della fase.
- Operator: L'operatore MongoDB da utilizzare (es. ```$match```, ```$project```).
- Args: Argomenti specifici per l'operatore.

#### Generazione della Pipeline

La funzione ```GenerateAggregation``` prende un'aggregazione e i parametri, e genera una pipeline MongoDB (mongo.Pipeline). Per ogni fase nella definizione dell'aggregazione, viene chiamata una funzione generatrice (GenerateStage) che costruisce la fase corrispondente in **bson.D**.

#### Configurazione delle aggregazioni

Le aggregazioni vengono definite e configurate tramite file di configurazione (es. JSON, YAML) e caricate nell'applicazione. Questo permette di definire e modificare le pipeline di aggregazione senza dover cambiare il codice sorgente.

Esempio di configurazione YAML:

```yaml
aggregations:
  - name: exampleAggregation
    collection: exampleCollection
    stages:
      - key: stage1
        operator: $match
        args:
          field: value
      - key: stage2
        operator: $project
        args:
          field: 1
```
