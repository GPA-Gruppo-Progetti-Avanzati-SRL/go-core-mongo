package authorization

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.uber.org/fx"

	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// roleFunctions mappa roleId -> set di functionId (memorizzati come map[string]struct{} per lookup O(1))
type AuthorizationLut struct {
	mu         sync.Mutex
	updating   atomic.Bool
	lastUpdate atomic.Value
	minRefresh time.Duration

	ls        *mongolks.LinkedService
	roleFuncs sync.Map // key: roleId string, value: map[string]struct{}
}

type roleFunctionsAggRes struct {
	RoleID    string   `bson:"_id" json:"_id"`
	Functions []string `bson:"functions" json:"functions"`
}

func NewAuthorizationLut(lc fx.Lifecycle, ls *mongolks.LinkedService) *AuthorizationLut {

	refresh := 10 * time.Minute

	l := &AuthorizationLut{

		ls:         ls,
		minRefresh: refresh,
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			l.lastUpdate.Store(time.Unix(0, 0))
			log.Info().Msgf("INIT Authorization LUT: refresh=%s", refresh)
			l.refresh()

			return nil
		},
	})

	return l
}

func (l *AuthorizationLut) expired(ignoreUpdating bool) bool {
	if !ignoreUpdating && l.updating.Load() {
		return false
	}
	last := l.lastUpdate.Load().(time.Time)
	return time.Now().Add(-l.minRefresh).After(last)
}

func (l *AuthorizationLut) refresh() *core.ApplicationError {

	l.mu.Lock()
	if l.updating.Load() {
		l.mu.Unlock()
		return nil
	}
	l.updating.Store(true)
	l.mu.Unlock()
	defer l.updating.Store(false)

	if !l.expired(true) {
		return nil
	}

	log.Info().Msg("Authorization LUT refresh start")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Esegue aggregazione inline (senza YAML): risultato [{ _id: roleId, functions: [..] }]

	collName := "acl"

	coll := l.ls.GetCollection(collName, "")

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"type": "role"}}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 1, "functiongroups": 1}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$functiongroups", "preserveNullAndEmptyArrays": false}}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from": collName,
			"let":  bson.M{"fgId": "$functiongroups"},
			"pipeline": mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.M{"$expr": bson.M{"$eq": bson.A{"$_id", "$$fgId"}}}}},
				bson.D{{Key: "$match", Value: bson.M{"type": "functiongroup"}}},
				bson.D{{Key: "$project", Value: bson.M{"_id": 1, "functions": 1}}},
			},
			"as": "functiongroup",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$functiongroup"}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$functiongroup.functions"}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":       "$_id",
			"functions": bson.M{"$addToSet": "$functiongroup.functions"},
		}}},
	}

	cur, aggErr := coll.Aggregate(ctx, pipeline)
	if aggErr != nil {
		log.Error().Err(aggErr).Msg("Authorization LUT aggregation error")
		return core.TechnicalErrorWithError(aggErr)
	}
	defer func() { _ = cur.Close(ctx) }()
	var res []*roleFunctionsAggRes
	if err := cur.All(ctx, &res); err != nil {
		log.Error().Err(err).Msg("Authorization LUT cursor error")
		return core.TechnicalErrorWithError(err)
	}

	// Popola mappa temporanea, poi sostituisce
	for _, r := range res {
		set := make(map[string]struct{}, len(r.Functions))
		for _, f := range r.Functions {
			set[f] = struct{}{}
		}
		l.roleFuncs.Store(r.RoleID, set)
	}
	l.lastUpdate.Store(time.Now())
	log.Info().Msgf("Authorization LUT refresh done: roles=%d", len(res))
	return nil
}

// Match implementa RoleMatcher.
func (l *AuthorizationLut) Match(roles []string, functionId string) bool {
	if l.expired(false) {
		go l.refresh()
	}
	for _, rid := range roles {
		if v, ok := l.roleFuncs.Load(rid); ok {
			set := v.(map[string]struct{})
			if _, okF := set[functionId]; okF {
				return true
			}
		}
	}
	return false
}
