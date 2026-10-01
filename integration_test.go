package coremongo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/page"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// I test di questo file girano contro un MongoDB vero. Le transazioni richiedono un replica set: MONGO_URL, es.
// mongodb://localhost:27017/?directConnection=true su un mongod avviato con --replSet (la stessa
// variabile della conformance di go-core-locker). Senza, i test sono saltati.
var txService = sync.OnceValues(func() (*Service, error) {
	var svc *Service
	Module(&Config{
		Name:   "txtest",
		Host:   os.Getenv("MONGO_URL"),
		DbName: fmt.Sprintf("coremongo_txtest_%d", time.Now().UnixNano()),
		Collections: mongolks.CollectionsCfg{
			{Id: "contatori", Name: "contatori"},
			{Id: "persone", Name: "persone"},
			{Id: "sequence", Name: "sequence"},
		},
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

type persona struct {
	Id   string `bson:"_id"`
	Nome string `bson:"nome"`
	Eta  int    `bson:"eta"`
}

func (persona) GetCollectionName(context.Context) string { return "persone" }

type personaPerId struct {
	Id string `field:"_id" operator:"$eq" omitempty:"true"`
}

func (personaPerId) GetFilterCollectionName(context.Context) string { return "persone" }

type tutteLePersone struct {
	Nome string `field:"nome" operator:"$eq" omitempty:"true"`
}

func (tutteLePersone) GetFilterCollectionName(context.Context) string { return "persone" }

// Un update che riscrive gli stessi valori è un successo: prima ModifiedCount = 0 diventava un 500
// "aggiornamento incoerente", e ogni PUT ripetuto falliva. Un documento assente è un 404.
func TestUpdate_IdempotenteENonTrovato(t *testing.T) {
	s := newTxService(t)
	ctx := context.Background()
	if _, err := s.GetCollection("persone", "").InsertOne(ctx, persona{Id: "p1", Nome: "Ada", Eta: 36}); err != nil {
		t.Fatal(err)
	}
	set := bson.M{"$set": bson.M{"eta": 37}}
	for i := range 2 {
		if err := s.UpdateOne(ctx, personaPerId{Id: "p1"}, set); err != nil {
			t.Fatalf("UpdateOne #%d: %v", i+1, err)
		}
	}
	if err := s.ReplaceOne[persona](ctx, personaPerId{Id: "p1"}, persona{Id: "p1", Nome: "Ada", Eta: 37}); err != nil {
		t.Fatalf("ReplaceOne a valori invariati: %v", err)
	}
	if err := s.UpdateMany(ctx, tutteLePersone{Nome: "Ada"}, set, 1); err != nil {
		t.Fatalf("UpdateMany a valori invariati: %v", err)
	}
	if err := s.UpdateSingleRecord(ctx, "persone", bson.M{"_id": "p1"}, set); err != nil {
		t.Fatalf("UpdateSingleRecord a valori invariati: %v", err)
	}
	err := s.UpdateOne(ctx, personaPerId{Id: "nessuno"}, set)
	if err == nil || err.StatusCode != 404 {
		t.Fatalf("UpdateOne su un documento assente: %v, atteso 404", err)
	}
	// Un filtro vuoto su UpdateSingleRecord non aggiorna un documento a caso.
	if err := s.UpdateSingleRecord(ctx, "persone", bson.M{}, set); err == nil {
		t.Fatal("UpdateSingleRecord con filtro vuoto: atteso errore")
	}
}

// Senza un sort del chiamante, le pagine sono ordinate per _id: altrimenti due pagine possono
// ripetere o saltare documenti.
func TestGetPageByFilter_OrdineStabile(t *testing.T) {
	s := newTxService(t)
	ctx := context.Background()
	coll := s.GetCollection("persone", "")
	for _, id := range []string{"e", "b", "d", "a", "c"} {
		if _, err := coll.InsertOne(ctx, persona{Id: id, Nome: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	var visti []string
	for pagina := 1; pagina <= 3; pagina++ {
		p := &page.Paging{CurrentPage: pagina, PageSize: 2}
		out, err := s.GetPageByFilter[persona](ctx, tutteLePersone{}, p)
		if err != nil {
			t.Fatalf("pagina %d: %v", pagina, err)
		}
		for _, o := range out {
			visti = append(visti, o.Id)
		}
	}
	if strings.Join(visti, "") != "abcde" {
		t.Fatalf("documenti per pagina = %v, attesi a..e in ordine e senza ripetizioni", visti)
	}
}

// Un contatore int64 (creato a mano o migrato) è una sequenza valida: prima solo l'int32 lo era.
func TestGetSequence_Int64(t *testing.T) {
	s := newTxService(t)
	ctx := context.Background()
	if _, err := s.GetCollection("sequence", "").InsertOne(ctx, bson.M{"_id": "grande", "sequence": int64(1) << 40}); err != nil {
		t.Fatal(err)
	}
	n, err := s.GetSequence(ctx, "sequence", "grande")
	if err != nil || n != 1<<40+1 {
		t.Fatalf("GetSequence = %d, %v", n, err)
	}
	if n, err = s.GetSequence(ctx, "sequence", "nuova"); err != nil || n != 1 {
		t.Fatalf("GetSequence su una sequenza nuova = %d, %v", n, err)
	}
}
