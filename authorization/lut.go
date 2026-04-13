package authorization

import (
	"context"
	"strings"
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

	roleApis     sync.Map // roleId -> map[string]ApiNode
	roleUis      sync.Map // roleId -> map[string]UINode
	roleActUi    sync.Map // roleId -> map[string]ActUi
	roleActApi   sync.Map // roleId -> map[string]ActApi
	apps         sync.Map // appId -> App
	contexts     sync.Map // contextId -> MongoContext
	roleContexts sync.Map // roleId -> contextId  (assente se il ruolo è context-agnostic)
}

type roleFunctionsAggRes struct {
	RoleID    string    `bson:"_id" json:"_id"`
	ContextID string    `bson:"cid" json:"cid"` // da _cid del role
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

	collName := "acl"
	coll := l.ls.GetCollection(collName, "")

	pipeline := mongo.Pipeline{
		// 1) Ruoli
		bson.D{{Key: "$match", Value: bson.M{"_et": "ROLE"}}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 1, "_cid": 1, "capability_groups": 1, "capabilities": 1}}},

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
			"_id":  1,
			"_cid": 1,
			"all_caps": bson.M{"$setUnion": bson.A{
				bson.M{"$ifNull": bson.A{"$capabilities", bson.A{}}},
				bson.M{"$ifNull": bson.A{"$cg_caps.capabilities", bson.A{}}},
			}},
		}}},

		// 3) Group per rimettere insieme i pezzi dei vari CG esplosi con unwind
		bson.D{{Key: "$group", Value: bson.M{
			"_id":      "$_id",
			"_cid":     bson.M{"$first": "$_cid"},
			"all_caps": bson.M{"$addToSet": "$all_caps"},
		}}},
		bson.D{{Key: "$project", Value: bson.M{
			"_id":  1,
			"_cid": 1,
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
					"menu":        "$ui.menu",
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
			"cid": bson.M{"$first": "$_cid"},
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

		// roleContexts: registra solo i ruoli con contesto
		if r.ContextID != "" {
			l.roleContexts.Store(r.RoleID, r.ContextID)
		}
	}

	l.lastUpdate.Store(time.Now())

	// Caricamento App catalog
	appCur, appErr := coll.Find(ctx, bson.M{"_et": "APP"})
	if appErr == nil {
		defer func() { _ = appCur.Close(ctx) }()
		for appCur.Next(ctx) {
			var a App
			if err := appCur.Decode(&a); err == nil && a.ID != "" {
				l.apps.Store(a.ID, a)
			}
		}
	}

	// Caricamento Context catalog
	ctxCur, ctxErr := coll.Find(ctx, bson.M{"_et": "CONTEXT"})
	if ctxErr == nil {
		defer func() { _ = ctxCur.Close(ctx) }()
		for ctxCur.Next(ctx) {
			var c MongoContext
			if err := ctxCur.Decode(&c); err == nil && c.ID != "" {
				l.contexts.Store(c.ID, c)
			}
		}
	}

	log.Info().Msgf("Authorization LUT refresh done: roles=%d", len(res))
	return nil
}

// AllContextIDs restituisce tutti gli ID di contesto presenti nella LUT (da MongoDB _et: CONTEXT).
// Usato dal gateway al boot per registrare le route /{cid}/*, indipendentemente dai ruoli utente.
func (l *AuthorizationLut) AllContextIDs() []string {
	var ids []string
	l.contexts.Range(func(key, _ interface{}) bool {
		ids = append(ids, key.(string))
		return true
	})
	return ids
}

// HomeAppForContext restituisce l'ID dell'app home designata per il contesto indicato.
func (l *AuthorizationLut) HomeAppForContext(contextID string) string {
	if val, ok := l.contexts.Load(contextID); ok {
		return val.(MongoContext).HomeApp
	}
	return ""
}

// FilterRolesByContext restituisce i ruoli validi per il contesto indicato.
// Sono inclusi i ruoli con _cid == contextId e i ruoli senza contesto (context-agnostic).
// Se contextId è vuoto restituisce tutti i ruoli invariati.
func (l *AuthorizationLut) FilterRolesByContext(roles []string, contextId string) []string {
	if contextId == "" {
		return roles
	}
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		cid, hasCid := l.roleContexts.Load(role)
		if !hasCid || cid.(string) == contextId {
			result = append(result, role)
		}
	}
	return result
}

// GetContexts restituisce i contesti accessibili per i ruoli passati.
// Deve ricevere allRoles (non filtrati) per fornire visione globale.
func (l *AuthorizationLut) GetContexts(roles []string) []*authcore.Context {
	if l.expired(false) {
		go l.refresh()
	}

	seen := make(map[string]struct{})
	for _, role := range roles {
		if cid, ok := l.roleContexts.Load(role); ok {
			seen[cid.(string)] = struct{}{}
		}
	}

	out := make([]*authcore.Context, 0, len(seen))
	for cid := range seen {
		if val, ok := l.contexts.Load(cid); ok {
			c := val.(MongoContext)
			out = append(out, &authcore.Context{
				ID:          c.ID,
				Description: c.Description,
				Label:       c.Label,
			})
		}
	}
	return out
}

// Match verifica l'autorizzazione per operationId (uso backend go-core-api).
func (l *AuthorizationLut) Match(roles []string, operationId string) bool {
	if l.expired(false) {
		go l.refresh()
	}
	for _, rid := range roles {
		if v, ok := l.roleApis.Load(rid); ok {
			for _, node := range v.(map[string]ApiNode) {
				if node.Api.OperationID == operationId {
					return true
				}
			}
		}
	}
	return false
}

// MatchRequest verifica l'autorizzazione per path HTTP + method (uso backend/gateway).
// Il campo Api.Path del ApiNode supporta glob: /api/persons/**, /api/persons/*.
// Se Api.OperationID è valorizzato e il path è vuoto, viene usato per il matching (fallback).
// Api.Methods vuoto su un nodo significa tutti i metodi.
func (l *AuthorizationLut) MatchRequest(roles []string, path, method string) bool {
	if l.expired(false) {
		go l.refresh()
	}
	for _, rid := range roles {
		if v, ok := l.roleApis.Load(rid); ok {
			for _, node := range v.(map[string]ApiNode) {
				if node.Api.Path != "" {
					if matchGlob(node.Api.Path, path) && matchMethods(node.Api.Methods, method) {
						return true
					}
				}
			}
		}
	}
	return false
}

// GetCapabilities restituisce la lista di capabilities abilitate per i ruoli.
// Applica il filtro appId solo alle categorie 'action_ui'.
func (l *AuthorizationLut) GetCapabilities(roles []string, appId string) []string {
	if l.expired(false) {
		go l.refresh()
	}

	outSet := make(map[string]struct{})
	for _, rid := range roles {
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

// GetServerCapabilities restituisce gli id delle capability di tipo "api" (server-side)
// abilitate per i ruoli. Non filtra per appId — le api capabilities non sono app-scoped.
func (l *AuthorizationLut) GetServerCapabilities(roles []string) []string {
	if l.expired(false) {
		go l.refresh()
	}
	outSet := make(map[string]struct{})
	for _, rid := range roles {
		if v, ok := l.roleApis.Load(rid); ok {
			for _, node := range v.(map[string]ApiNode) {
				outSet[node.ID] = struct{}{}
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

// GetPaths restituisce un elenco PIATTO dei menu autorizzati per i ruoli passati.
// Se viene passato un appId, i menu vengono filtrati strettamente per appId.
func (l *AuthorizationLut) GetPaths(roles []string, appId string) []*authcore.Path {
	if l.expired(false) {
		go l.refresh()
	}

	menusMap := make(map[string]*UINode)
	for _, rid := range roles {
		if v, ok := l.roleUis.Load(rid); ok {
			for id, node := range v.(map[string]UINode) {
				if node.AppID == appId {
					menusMap[id] = &node
				}
			}
		}
	}

	out := make([]*authcore.Path, 0, len(menusMap))
	for _, f := range menusMap {
		out = append(out, &authcore.Path{
			ID:          f.ID,
			Description: f.Description,
			Icon:        f.Icon,
			Order:       f.Order,
			Endpoint:    f.Endpoint,
			Menu:        f.IsMenu,
		})
	}
	return out
}

// GetApps restituisce le app navigabili per i ruoli e il contesto indicati.
//
// L'home app (BasePath == "/") è sempre inclusa:
//   - senza contesto → path "/"
//   - con contesto   → path "/{contextID}/"
//
// Le altre app sono incluse solo se contextID è valorizzato e l'utente ha ruoli
// (filtrati per contesto) che hanno almeno un UI node per quell'app.
// I path diventano "/{contextID}/{basePath}".
//
// Deve ricevere allRoles (non filtrati per contesto).
func (l *AuthorizationLut) GetApps(roles []string, contextID string) []*authcore.App {
	if l.expired(false) {
		go l.refresh()
	}

	// Ruoli filtrati per contesto: usati per determinare le app accessibili
	ctxRoles := roles
	if contextID != "" {
		ctxRoles = l.FilterRolesByContext(roles, contextID)
	}

	// App accessibili tramite UI nodes dei ruoli filtrati
	accessible := make(map[string]struct{})
	for _, rid := range ctxRoles {
		if v, ok := l.roleUis.Load(rid); ok {
			for _, node := range v.(map[string]UINode) {
				if node.AppID != "" {
					accessible[node.AppID] = struct{}{}
				}
			}
		}
	}

	// Modalità no-context: nessun ruolo dell'utente ha un _cid.
	// In questo caso tutte le app sono esposte direttamente (path originali, nessun prefisso).
	noContextMode := contextID == ""
	if noContextMode {
		for _, rid := range roles {
			if _, ok := l.roleContexts.Load(rid); ok {
				noContextMode = false
				break
			}
		}
	}

	cid := strings.ToLower(contextID)
	out := make([]*authcore.App, 0)

	l.apps.Range(func(_, val interface{}) bool {
		a := val.(App)
		isHome := a.BasePath == "/"

		if !isHome {
			if contextID == "" && !noContextMode {
				return true // senza contesto le app non-home non vengono esposte
			}
			if _, ok := accessible[a.ID]; !ok {
				return true // non accessibile tramite ruoli
			}
		}

		path := a.BasePath
		appCtxID := ""
		if contextID != "" {
			if isHome {
				path = "/" + cid + "/"
			} else {
				path = "/" + cid + a.BasePath
			}
			appCtxID = contextID
		}

		out = append(out, &authcore.App{
			ID:          a.ID,
			Description: a.Description,
			Path:        path,
			Icon:        a.Icon,
			Order:       a.Order,
			ContextID:   appCtxID,
		})
		return true
	})

	return out
}

// / matchGlob matcha un pattern HTTP contro un path reale.
// Supporta:
//   - /api/**              → qualsiasi path che inizia con /api/
//   - /api/persons/*       → segmento singolo jolly: /api/persons/123
//   - /api/persons/:id     → path param nominale (singolo segmento)
//   - /api/persons/{id}    → path param nominale stile OpenAPI (singolo segmento)
//   - /api/persons/:id/orders → param in mezzo al path
//   - /api/persons         → match esatto
func matchGlob(pattern, path string) bool {
	if pattern == path {
		return true
	}
	// /** alla fine: prefisso libero
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	// match segmento per segmento
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	rp := strings.Split(strings.Trim(path, "/"), "/")
	if len(pp) != len(rp) {
		return false
	}
	for i := range pp {
		seg := pp[i]
		if seg == "*" || strings.HasPrefix(seg, ":") ||
			(strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")) {
			continue // wildcard o path param: matcha qualsiasi valore
		}
		if seg != rp[i] {
			return false
		}
	}
	return true
}

// matchMethods restituisce true se method è nella lista (case-insensitive) o se la lista è vuota.
func matchMethods(methods []string, method string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, m := range methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}
