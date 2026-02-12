package authorization

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	authcore "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/authorization"
	coremongo "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/fx"
)

type AuthorizationLut struct {
	mu         sync.Mutex
	updating   atomic.Bool
	lastUpdate atomic.Value
	minRefresh time.Duration
	ls         *mongolks.LinkedService

	roleApis   sync.Map // roleId -> map[string]ApiNode
	roleUis    sync.Map // roleId -> map[string]UINode
	roleActUi  sync.Map // roleId -> map[string]ActUi
	roleActApi sync.Map // roleId -> map[string]ActApi
}

type roleFunctionsAggRes struct {
	RoleID    string    `bson:"_id" json:"_id"`
	Apis      []ApiNode `bson:"apis" json:"apis"`
	UIs       []UINode  `bson:"uis" json:"uis"`
	ActionUI  []ActUi   `bson:"actsUI" json:"actsUI"`
	ActionApi []ActApi  `bson:"actsApi" json:"actsApi"`
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
		// 1) Ruoli
		bson.D{{Key: "$match", Value: bson.M{"_et": "ROLE"}}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 1, "capability_groups": 1, "capabilities": 1}}},

		// 2a) Unwind capability_groups e lookup
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$capability_groups", "preserveNullAndEmptyArrays": true}}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from": collName,
			"let":  bson.M{"cgId": "$capability_groups"},
			"pipeline": mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.M{"$expr": bson.M{"$eq": bson.A{"$_id", "$$cgId"}}}}},
				bson.D{{Key: "$match", Value: bson.M{"_et": "CAPABILITYGROUP"}}},
				bson.D{{Key: "$project", Value: bson.M{"_id": 0, "capabilities": 1}}},
			},
			"as": "cg_caps",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$cg_caps", "preserveNullAndEmptyArrays": true}}},

		// Combinazione capabilities dirette + quelle del gruppo
		bson.D{{Key: "$project", Value: bson.M{
			"_id": 1,
			"all_caps": bson.M{"$setUnion": bson.A{
				bson.M{"$ifNull": bson.A{"$capabilities", bson.A{}}},
				bson.M{"$ifNull": bson.A{"$cg_caps.capabilities", bson.A{}}},
			}},
		}}},

		// 3) Group per rimettere insieme i pezzi dei vari CG esplosi con unwind
		bson.D{{Key: "$group", Value: bson.M{
			"_id":      "$_id",
			"all_caps": bson.M{"$addToSet": "$all_caps"},
		}}},
		bson.D{{Key: "$project", Value: bson.M{
			"_id": 1,
			"all_caps": bson.M{"$reduce": bson.M{
				"input":        "$all_caps",
				"initialValue": bson.A{},
				"in":           bson.M{"$setUnion": bson.A{"$$value", "$$this"}},
			}},
		}}},

		bson.D{{Key: "$unwind", Value: bson.M{"path": "$all_caps", "preserveNullAndEmptyArrays": false}}},

		// 4) Lookup capability per leggere i campi nested utili
		bson.D{{Key: "$lookup", Value: bson.M{
			"from": collName,
			"let":  bson.M{"capId": "$all_caps"},
			"pipeline": mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.M{"$expr": bson.M{"$eq": bson.A{"$_id", "$$capId"}}}}},
				bson.D{{Key: "$match", Value: bson.M{"_et": "CAPABILITY"}}},
				bson.D{{Key: "$project", Value: bson.M{
					"_id":         1,
					"category":    1,
					"appId":       1,
					"operationid": "$api.operationid",
					"icon":        "$ui.icon",
					"order":       "$ui.order",
					"endpoint":    "$ui.endpoint",
					"description": 1,
				}}},
			},
			"as": "capability",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$capability"}}},

		// 5) Group per ruolo con insiemi distinti per categorie
		bson.D{{Key: "$group", Value: bson.M{
			"_id": "$_id",
			"apis": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$capability.category", "api"}},
					"$capability",
					"$$REMOVE",
				},
			}},
			"uis": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$capability.category", "ui"}},
					"$capability",
					"$$REMOVE",
				},
			}},
			"actsUI": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$capability.category", "action_ui"}},
					"$capability",
					"$$REMOVE",
				},
			}},
			"actsApi": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$capability.category", "action_api"}},
					"$capability",
					"$$REMOVE",
				},
			}},
		}}},
	}
	if zerolog.GlobalLevel() < zerolog.DebugLevel {
		log.Trace().Msgf("%s", coremongo.PipelineToJson(pipeline))
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

	// Popola mappe per viste
	for _, r := range res {
		// Apis
		apiMap := make(map[string]ApiNode)
		for _, v := range r.Apis {
			if v.ID != "" {
				apiMap[v.ID] = v
			}
		}
		l.roleApis.Store(r.RoleID, apiMap)

		// UIs
		uiMap := make(map[string]UINode)
		for _, v := range r.UIs {
			if v.ID != "" {
				uiMap[v.ID] = v
			}
		}
		l.roleUis.Store(r.RoleID, uiMap)

		// ActsUI
		actUiMap := make(map[string]ActUi)
		for _, v := range r.ActionUI {
			if v.ID != "" {
				actUiMap[v.ID] = v
			}
		}
		l.roleActUi.Store(r.RoleID, actUiMap)

		// ActsApi
		actApiMap := make(map[string]ActApi)
		for _, v := range r.ActionApi {
			if v.ID != "" {
				actApiMap[v.ID] = v
			}
		}
		l.roleActApi.Store(r.RoleID, actApiMap)
	}

	l.lastUpdate.Store(time.Now())
	log.Info().Msgf("Authorization LUT refresh done: roles=%d", len(res))
	return nil
}

// Match implementa RoleMatcher per endpoint (operationId).
func (l *AuthorizationLut) Match(roles []string, operationId string) bool {
	if l.expired(false) {
		go l.refresh()
	}
	for _, rid := range roles {
		if v, ok := l.roleApis.Load(rid); ok {
			for _, node := range v.(map[string]ApiNode) {
				if node.OperationID == operationId {
					return true
				}
			}
		}
	}
	return false
}

// GetCapabilities restituisce la lista di capabilities abilitate per i ruoli.
// Applica il filtro appId solo alle categorie 'ui' e 'action_ui'.
func (l *AuthorizationLut) GetCapabilities(roles []string, appId string) []string {
	if l.expired(false) {
		go l.refresh()
	}

	outSet := make(map[string]struct{})
	for _, rid := range roles {
		// 3. Action UI (con filtro appId)
		if v, ok := l.roleActUi.Load(rid); ok {
			for _, node := range v.(map[string]ActUi) {
				if node.AppID == "" || node.AppID == appId {
					outSet[node.ID] = struct{}{}
				}
			}
		}
	}

	out := make([]string, 0, len(outSet))
	for k := range outSet {
		out = append(out, k)
	}
	return out
}

// HasCapability verifica se almeno uno dei ruoli possiede la capability indicata.
func (l *AuthorizationLut) HasCapability(roles []string, capabilityId string) bool {
	if l.expired(false) {
		go l.refresh()
	}

	for _, rid := range roles {
		if v, ok := l.roleActApi.Load(rid); ok {
			if _, found := v.(map[string]ActApi)[capabilityId]; found {
				return true
			}
		}
	}
	return false
}

// GetMenu restituisce un elenco PIATTO dei menu autorizzati per i ruoli passati.
// Se viene passato un appId, i menu vengono filtrati strettamente per appId.
func (l *AuthorizationLut) GetMenu(roles []string, appId string) []*authcore.MenuNode {
	if l.expired(false) {
		go l.refresh()
	}

	// Mappa per evitare duplicati di menu tra ruoli diversi
	menusMap := make(map[string]*UINode)
	for _, rid := range roles {
		if v, ok := l.roleUis.Load(rid); ok {
			for id, node := range v.(map[string]UINode) {
				// Filtro stretto: includi solo voci con appid esattamente uguale a filterApp
				if node.AppID == appId {
					menusMap[id] = &node
				}
			}
		}
	}

	out := make([]*authcore.MenuNode, 0, len(menusMap))
	for _, f := range menusMap {
		n := &authcore.MenuNode{
			ID:          f.ID,
			Description: f.Description,
			Icon:        f.Icon,
			Order:       f.Order,
			Endpoint:    f.Endpoint,
		}
		out = append(out, n)
	}
	return out
}
