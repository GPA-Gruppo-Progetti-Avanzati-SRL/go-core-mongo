package coremongo

import (
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/page"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SortToBson converts a SortRequest to a bson.D.
// bson.D preserves insertion order, which is required for multi-field sorts.
func SortToBson(s page.SortRequest) bson.D {
	d := make(bson.D, 0, len(s))
	for _, f := range s {
		d = append(d, bson.E{Key: f.Field, Value: int(f.Dir)})
	}
	return d
}

// FindSortOption converts a SortRequest into a FindOptions lister
// ready to be passed as a variadic argument to GetPageByFilter or any Find call.
//
// Usage:
//
//	results, err := coremongo.GetPageByFilter(ctx, ms, filter, paging, coremongo.FindSortOption(sortReq))
func FindSortOption(s page.SortRequest) options.Lister[options.FindOptions] {
	return options.Find().SetSort(SortToBson(s))
}
