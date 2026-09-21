// Package mongoutil raccoglie le utility neutre attorno al driver Mongo.
//
// È un package foglia — non importa nulla di go-core-mongo — perché i suoi utenti stanno sui due
// lati di una dipendenza che non si può invertire: il package root `coremongo` importa
// `authorization` (per esporne l'Option), quindi `authorization` non può importare il root, e una
// utility al root sarebbe inutilizzabile proprio dove i cursori sono tre.
package mongoutil

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// CloseCursor chiude il cursore e logga l'esito negativo. Una Close fallita non offre
// un'alternativa da scegliere — la lettura è già finita — ma è l'unico segnale che quella query
// non si è chiusa pulita: scartarla con `_ =` lo cancella. `what` nomina il sito che stava
// leggendo, altrimenti il warning non dice da dove viene.
//
// Uso: `defer mongoutil.CloseCursor(ctx, cur, "GetObjectsByFilter")`.
func CloseCursor(ctx context.Context, cur *mongo.Cursor, what string) {
	if err := cur.Close(ctx); err != nil {
		log.Warn().Err(err).Str("query", what).Msg("chiusura del cursore fallita")
	}
}
