package coremongo

import (
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
)

// Ambit è la libreria di origine dell'errore. I costruttori di core mettono in Ambit l'AppName,
// cioè l'applicazione che l'errore lo *riceve*: senza sovrascriverlo un guasto del driver Mongo
// si presenta come un errore dell'app, e chi legge il log non sa in quale libreria guardare.
const Ambit = "go-core-mongo"

// Codici degli errori emessi dal modulo. Il prefisso dice la famiglia:
//   - MONGO-*  operazione sul driver, o risorsa non configurata
//   - MON-*    incoerenza semantica rilevata dalla libreria (storici, mantenuti)
const (
	CodeCollectionNotFound  = "MONGO-COLL-NOTFOUND" // collection non presente in mongo.collections
	CodeAggregationNotFound = "MONGO-AGGR-NOTFOUND" // aggregation non presente nel registry
	CodeAggregationOperator = "MONGO-AGGR-OP"       // stage con operatore non supportato
	CodeFilter              = "MONGO-FILTER"        // buildFilter fallita: tag field/operator non validi
	CodeFindOne             = "MONGO-FINDONE"       // FindOne fallita (o decode del singolo documento)
	CodeFind                = "MONGO-FIND"          // Find fallita
	CodeCursor              = "MONGO-CURSOR"        // iterazione/decode del cursore fallita
	CodeCount               = "MONGO-COUNT"         // CountDocuments fallita
	CodeInsert              = "MONGO-INSERT"        // InsertOne/InsertMany fallita
	CodeInsertNoID          = "MONGO-INSERT-NOID"   // insert riuscita ma senza InsertedID
	CodeUpdate              = "MONGO-UPDATE"        // UpdateOne/UpdateMany fallita
	CodeReplace             = "MONGO-REPLACE"       // ReplaceOne fallita
	CodeDelete              = "MONGO-DELETE"        // DeleteOne/DeleteMany fallita
	CodeTransaction         = "MONGO-TX"            // sessione o transazione fallita
	CodeSequence            = "MONGO-SEQ"           // lettura della sequenza fallita
	CodeBulk                = "MONGO-BULK"          // BulkWrite fallita
	CodeExecAggregation     = "MONGO-EXECAGGR"      // esecuzione della pipeline fallita
	CodeExecAggregationCur  = "MONGO-EXECAGGR-CUR"  // cursore dell'aggregation fallito
	CodeGetObjectsFind      = "MONGO-GOBF-ERRFIND"  // storico: Find in GetObjectsByFilter
	CodeGetObjectsCursor    = "MONGO-GOBF-ERRCUR"   // storico: cursore in GetObjectsByFilter
	CodeGetObjectsSorted    = "MONGO-GOBFS-ERRFIND" // storico: Find/cursore nella variante sorted
	CodeProperties          = "PROPERTIES"          // unmarshal di filtro/sort dalle properties
	CodeInsertMismatch      = "INSERT-MISMATCH"     // InsertMany: inseriti != richiesti
	CodeInconsistent        = "MON-AGGINC"          // update/delete che ha toccato != 1 documento
	CodeAggregationFilter   = "MON-FIL"             // filtro dell'aggregation non è un IFilter
	CodeAggregationSort     = "MON-SOR"             // sort dell'aggregation malformato
	CodeSequenceInvalid     = "SEQ-INV"             // campo sequence non intero
)

// techErr è il costruttore usato da tutto il modulo: un errore tecnico che dichiara sempre
// codice e libreria di origine. Esiste perché l'ambit era l'unica cosa che si poteva
// dimenticare su ognuno dei ~50 siti di errore, e dimenticarla non rompe niente — semplicemente
// attribuisce il guasto all'app.
func techErr(code string) *core.ApplicationError {
	return core.TechnicalError().WithAmbit(Ambit).WithCode(code)
}

// notFound è il 404 del modulo (codice NOT-FOUND di core), con la libreria di origine.
func notFound() *core.ApplicationError {
	return core.NotFoundError().WithAmbit(Ambit)
}
