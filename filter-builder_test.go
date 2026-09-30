package coremongo

import (
	"context"
	"testing"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type filtroConInterno struct {
	cache string // non esportato, senza tag: prima faceva panicare buildFilter
	Nome  string `field:"name" operator:"$eq"`
	Eta   int    `field:"age" operator:"$gt" omitempty:""`
}

func (filtroConInterno) GetFilterCollectionName(context.Context) string { return "persone" }

func TestBuildFilter_CampoNonEsportatoNonPanica(t *testing.T) {
	f, err := buildFilter(&filtroConInterno{cache: "x", Nome: "ada"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f["name"]; !ok || len(f) != 1 {
		t.Fatalf("filtro = %v, atteso solo name (age omesso perché zero)", f)
	}
}

type filtroTesto struct {
	Starts  string `field:"a" operator:"$startswith" omitempty:""`
	IStarts string `field:"b" operator:"$istartswith" omitempty:""`
	Ends    string `field:"c" operator:"$endswith" omitempty:""`
	Cont    string `field:"d" operator:"$icontains" omitempty:""`
	Regex   string `field:"e" operator:"$regex" omitempty:""`
}

func (filtroTesto) GetFilterCollectionName(context.Context) string { return "x" }

// TestBuildFilter_OperatoriTestualiEscapano: $startswith/$endswith/$contains confrontano un testo,
// quindi i metacaratteri del valore sono letterali. Prima `.*` o `(a+)+$` arrivati da un query param
// diventavano pattern: un filtro allargato o una ReDoS sul database. $regex invece resta un pattern.
func TestBuildFilter_OperatoriTestualiEscapano(t *testing.T) {
	f, err := buildFilter(&filtroTesto{Starts: ".*", IStarts: "a+b", Ends: "(x)", Cont: "a|b", Regex: "^ab.*"})
	if err != nil {
		t.Fatal(err)
	}
	if got := f["a"].(bson.M)["$regex"]; got != `^\.\*` {
		t.Errorf("$startswith = %v", got)
	}
	if got := f["b"].(bson.M)["$regex"].(bson.Regex).Pattern; got != `^a\+b` {
		t.Errorf("$istartswith = %v", got)
	}
	if got := f["c"].(bson.M)["$regex"]; got != `\(x\)$` {
		t.Errorf("$endswith = %v", got)
	}
	if got := f["d"].(bson.M)["$regex"].(bson.Regex).Pattern; got != `a\|b` {
		t.Errorf("$icontains = %v", got)
	}
	if got := f["e"].(bson.M)["$regex"]; got != "^ab.*" {
		t.Errorf("$regex deve restare un pattern: %v", got)
	}
}

type docVuoto struct{}

func (docVuoto) GetCollectionName(context.Context) string { return "x" }

// TestScrittureConFiltroVuoto: un filtro coi campi omitempty tutti vuoti è `{}`. Per una lettura
// è "tutto", per una scrittura è un errore che arriva prima di toccare il database — qui un Service
// vuoto, che panicherebbe se la guardia non ci fosse. Prima UpdateMany/DeleteMany lavoravano
// sull'intera collection e UpdateOne/ReplaceOne/DeleteOne su un documento qualsiasi.
func TestScrittureConFiltroVuoto(t *testing.T) {
	s := &Service{}
	ctx := t.Context()
	empty := &filtroTesto{}
	for name, call := range map[string]func() *core.Error{
		"UpdateOne":  func() *core.Error { return s.UpdateOne(ctx, empty, bson.M{"$set": bson.M{"a": 1}}) },
		"UpdateMany": func() *core.Error { return s.UpdateMany(ctx, empty, bson.M{"$set": bson.M{"a": 1}}, 1) },
		"ReplaceOne": func() *core.Error { return s.ReplaceOne[docVuoto](ctx, empty, docVuoto{}) },
		"DeleteOne":  func() *core.Error { return s.DeleteOne(ctx, empty) },
		"DeleteMany": func() *core.Error { return s.DeleteMany(ctx, empty) },
	} {
		if appErr := call(); appErr == nil || appErr.Code != CodeEmptyFilter || appErr.StatusCode != 422 {
			t.Errorf("%s: atteso %s (422), ottenuto %v", name, CodeEmptyFilter, appErr)
		}
	}
}
