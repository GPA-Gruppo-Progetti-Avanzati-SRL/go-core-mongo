package coremongo

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Le transazioni richiedono un replica set vero: MONGO_URL, es.
// mongodb://localhost:27017/?directConnection=true su un mongod avviato con --replSet (la stessa
// variabile della conformance di go-core-locker). Senza, i test sono saltati.
var txService = sync.OnceValues(func() (*Service, error) {
	var svc *Service
	Module(&Config{
		Name:                   "txtest",
		Host:                   os.Getenv("MONGO_URL"),
		DbName:                 fmt.Sprintf("coremongo_txtest_%d", time.Now().UnixNano()),
		Collections:            mongolks.CollectionsCfg{{Id: "contatori", Name: "contatori"}},
		ServerSelectionTimeout: 3 * time.Second,
	})
	core.Populate(&svc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := core.Start(ctx); err != nil {
		return nil, err
	}
	return svc, nil
})

func newTxService(t *testing.T) *Service {
	t.Helper()
	if os.Getenv("MONGO_URL") == "" {
		t.Skip("MONGO_URL non impostata: le transazioni richiedono un replica set")
	}
	s, err := txService()
	if err != nil {
		t.Fatalf("MongoDB: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Db().Drop(context.Background()); err != nil {
			t.Logf("drop del database di test: %v", err)
		}
	})
	return s
}

func contatore(t *testing.T, s *Service, id string) int {
	t.Helper()
	var doc struct {
		N int `bson:"n"`
	}
	if err := s.GetCollection("contatori", "").FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("FindOne(%s): %v", id, err)
	}
	return doc.N
}

func TestExecTransaction_CommitERollback(t *testing.T) {
	s := newTxService(t)
	ctx := context.Background()
	coll := s.GetCollection("contatori", "")
	if _, err := coll.InsertOne(ctx, bson.M{"_id": "c", "n": 0}); err != nil {
		t.Fatal(err)
	}
	inc := func(ctx context.Context) error {
		_, err := coll.UpdateOne(ctx, bson.M{"_id": "c"}, bson.M{"$inc": bson.M{"n": 1}})
		return err
	}

	if err := s.ExecTransaction(ctx, inc); err != nil {
		t.Fatalf("commit: %v", err)
	}
	boom := fmt.Errorf("boom")
	err := s.ExecTransaction(ctx, func(ctx context.Context) error {
		if err := inc(ctx); err != nil {
			return err
		}
		return boom
	})
	if err == nil {
		t.Fatal("un errore del callback deve risalire")
	}
	if n := contatore(t, s, "c"); n != 1 {
		t.Fatalf("n = %d, atteso 1: la seconda transazione andava abortita", n)
	}
}

// Un conflitto di scrittura è un TransientTransactionError: il driver sa che rieseguire la
// transazione per intero è sicuro, e prima ExecTransaction lo faceva risalire come errore.
func TestExecTransaction_RieseguitaSuConflittoDiScrittura(t *testing.T) {
	s := newTxService(t)
	ctx := context.Background()
	coll := s.GetCollection("contatori", "")
	if _, err := coll.InsertOne(ctx, bson.M{"_id": "c", "n": 0}); err != nil {
		t.Fatal(err)
	}

	attempts := 0
	err := s.ExecTransaction(ctx, func(txCtx context.Context) error {
		attempts++
		// La lettura fissa lo snapshot della transazione.
		if err := coll.FindOne(txCtx, bson.M{"_id": "c"}).Err(); err != nil {
			return err
		}
		if attempts == 1 {
			// Un'altra scrittura, committata dopo lo snapshot: la nostra update andrà in conflitto.
			if _, err := coll.UpdateOne(ctx, bson.M{"_id": "c"}, bson.M{"$inc": bson.M{"n": 10}}); err != nil {
				return err
			}
		}
		_, err := coll.UpdateOne(txCtx, bson.M{"_id": "c"}, bson.M{"$inc": bson.M{"n": 1}})
		return err
	})
	if err != nil {
		t.Fatalf("ExecTransaction: %v (tentativi: %d)", err, attempts)
	}
	if attempts != 2 {
		t.Fatalf("tentativi = %d, attesi 2 (il primo in conflitto)", attempts)
	}
	if n := contatore(t, s, "c"); n != 11 {
		t.Fatalf("n = %d, atteso 11", n)
	}
}
