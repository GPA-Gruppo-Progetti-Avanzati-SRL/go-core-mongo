package coremongo

import (
	"context"
	"testing"
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
