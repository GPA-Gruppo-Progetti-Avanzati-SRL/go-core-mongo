package coremongo

import "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"

// Config è la configurazione della connessione Mongo.
//
// È un alias di mongolks.Config: l'app dichiara `Mongo coremongo.Config` nella
// propria config e non deve importare tpm-mongo-common. La parte aggregation non
// vive qui: il path dei file è un fatto di compile-time, non di ambiente, quindi
// la cartella embeddata si passa a Module con WithAggregations.
type Config = mongolks.Config
