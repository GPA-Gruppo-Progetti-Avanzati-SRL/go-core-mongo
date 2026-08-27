package coremongo

import (
	"context"
	"testing"
)

// documentoDiProva è il minimo che soddisfa ICollection: BulkWrite legge il nome della collezione da
// uno zero value del type param.
type documentoDiProva struct{}

func (documentoDiProva) GetCollectionName(ctx context.Context) string { return "collezione" }

// Con una slice di WriteModel vuota, BulkWrite deve tornare subito senza toccare la LinkedService: un
// *Service con LinkedService nil (GetCollection andrebbe in panic) è quindi sufficiente a verificarlo,
// senza una connessione Mongo reale.
func TestBulkWrite_NessunModelloNessunAccessoAllaConnessione(t *testing.T) {
	s := &Service{}

	result, appErr := s.BulkWrite[documentoDiProva](t.Context(), nil)
	if appErr != nil {
		t.Fatalf("errore inatteso: %v", appErr)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil (nessuna BulkWrite eseguita)", result)
	}
}
