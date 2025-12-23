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

	ls *mongolks.LinkedService
	// Mappe per viste logiche: endpoint/capability/menu
	roleEndpoints          sync.Map // roleId -> set(operationId)
	roleClientCapabilities sync.Map // roleId -> set(capabilityId) (captype==client)
	roleServerCapabilities sync.Map // roleId -> set(capabilityId) (captype==server)
	roleMenus              sync.Map // roleId -> set(menuId)

	// Catalogo completo dei menu per costruire l'albero
	menuCatalog map[string]*Function // key: menuId
	// Catalogo capability per filtrare per appId (solo client)
	capCatalog map[string]*Function // key: capabilityId
}

type roleFunctionsAggRes struct {
	RoleID     string   `bson:"_id" json:"_id"`
	Endpoints  []string `bson:"endpoints" json:"endpoints"`
	ClientCaps []string `bson:"clientcaps" json:"clientcaps"`
	ServerCaps []string `bson:"servercaps" json:"servercaps"`
	Menus      []string `bson:"menus" json:"menus"`
}

func NewAuthorizationLut(lc fx.Lifecycle, ls *mongolks.LinkedService) *AuthorizationLut {

	refresh := 10 * time.Minute

	l := &AuthorizationLut{
		ls:          ls,
		minRefresh:  refresh,
		menuCatalog: make(map[string]*Function),
		capCatalog:  make(map[string]*Function),
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
		bson.D{{Key: "$match", Value: bson.M{"type": "role"}}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 1, "functiongroups": 1}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$functiongroups", "preserveNullAndEmptyArrays": false}}},
		// 2) Lookup functiongroup per ottenere l'array di functions (ids)
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
		// 3) Lookup function per leggere i campi nested utili (senza usare kind)
		bson.D{{Key: "$lookup", Value: bson.M{
			"from": collName,
			"let":  bson.M{"funcId": "$functiongroup.functions"},
			"pipeline": mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.M{"$expr": bson.M{"$eq": bson.A{"$_id", "$$funcId"}}}}},
				bson.D{{Key: "$match", Value: bson.M{"type": "function"}}},
				bson.D{{Key: "$project", Value: bson.M{"_id": 1, "endpoint.operationid": 1, "capability.captype": 1, "menu": 1}}},
			},
			"as": "function",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$function"}}},
		// 4) Group per ruolo con insiemi distinti per categorie
		bson.D{{Key: "$group", Value: bson.M{
			"_id": "$_id",
			"endpoints": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$ne": bson.A{bson.M{"$type": "$function.endpoint"}, "missing"}},
					"$function.endpoint.operationid",
					"$$REMOVE",
				},
			}},
			"clientcaps": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$and": bson.A{
						bson.M{"$ne": bson.A{bson.M{"$type": "$function.capability"}, "missing"}},
						bson.M{"$eq": bson.A{"$function.capability.captype", "client"}},
					}},
					"$function._id",
					"$$REMOVE",
				},
			}},
			"servercaps": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$and": bson.A{
						bson.M{"$ne": bson.A{bson.M{"$type": "$function.capability"}, "missing"}},
						bson.M{"$eq": bson.A{"$function.capability.captype", "server"}},
					}},
					"$function._id",
					"$$REMOVE",
				},
			}},
			"menus": bson.M{"$addToSet": bson.M{
				"$cond": bson.A{
					bson.M{"$ne": bson.A{bson.M{"$type": "$function.menu"}, "missing"}},
					"$function._id",
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
		// Endpoints
		if r.Endpoints != nil {
			set := make(map[string]struct{}, len(r.Endpoints))
			for _, v := range r.Endpoints {
				if v == "" {
					continue
				}
				set[v] = struct{}{}
			}
			l.roleEndpoints.Store(r.RoleID, set)
		}

		// Client Capabilities
		if r.ClientCaps != nil {
			set := make(map[string]struct{}, len(r.ClientCaps))
			for _, v := range r.ClientCaps {
				if v == "" {
					continue
				}
				set[v] = struct{}{}
			}
			l.roleClientCapabilities.Store(r.RoleID, set)
		}
		// Server Capabilities
		if r.ServerCaps != nil {
			set := make(map[string]struct{}, len(r.ServerCaps))
			for _, v := range r.ServerCaps {
				if v == "" {
					continue
				}
				set[v] = struct{}{}
			}
			l.roleServerCapabilities.Store(r.RoleID, set)
		}
		// Menus
		if r.Menus != nil {
			set := make(map[string]struct{}, len(r.Menus))
			for _, v := range r.Menus {
				if v == "" {
					continue
				}
				set[v] = struct{}{}
			}
			l.roleMenus.Store(r.RoleID, set)
		}
	}

	// Carica intero catalogo menu per costruire l'albero
	if err := l.loadMenuCatalog(ctx, collName); err != nil {
		log.Error().Err(err).Msg("Authorization LUT load menu catalog error")
		return err
	}
	// Carica catalogo capability per filtrare per app
	if err := l.loadCapabilityCatalog(ctx, collName); err != nil {
		log.Error().Err(err).Msg("Authorization LUT load capability catalog error")
		return err
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
		if v, ok := l.roleEndpoints.Load(rid); ok {
			set := v.(map[string]struct{})
			if _, okF := set[operationId]; okF {
				return true
			}
		}
	}
	return false
}

// GetClientCapabilities restituisce la lista di capabilities di tipo "client" abilitate per i ruoli.
func (l *AuthorizationLut) GetCapabilities(roles []string, appId ...string) []string {
	if l.expired(false) {
		go l.refresh()
	}
	var filterApp string
	if len(appId) > 0 {
		filterApp = appId[0]
	}
	outSet := make(map[string]struct{})
	for _, rid := range roles {
		if v, ok := l.roleClientCapabilities.Load(rid); ok {
			for k := range v.(map[string]struct{}) {
				// Se non filtriamo per app, includiamo tutto
				if filterApp == "" {
					outSet[k] = struct{}{}
					continue
				}
				// Se filtriamo per app: includi solo le capability con appid esattamente uguale
				if f, okF := l.capCatalog[k]; okF {
					if f.Capability != nil && f.Capability.AppID == filterApp {
						outSet[k] = struct{}{}
					}
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

// HasServerCapability verifica se almeno uno dei ruoli possiede la capability server indicata.
func (l *AuthorizationLut) HasCapability(roles []string, capabilityId string) bool {
	if l.expired(false) {
		go l.refresh()
	}
	for _, rid := range roles {
		if v, ok := l.roleServerCapabilities.Load(rid); ok {
			set := v.(map[string]struct{})
			if _, okF := set[capabilityId]; okF {
				return true
			}
		}
	}
	return false
}

// GetMenu costruisce l'albero dei menu autorizzati per i ruoli passati.
func (l *AuthorizationLut) GetMenu(roles []string, appId ...string) []*authcore.MenuNode {
	if l.expired(false) {
		go l.refresh()
	}
	var filterApp string
	if len(appId) > 0 {
		filterApp = appId[0]
	}
	// Unione dei menu autorizzati per i ruoli
	allowed := make(map[string]struct{})
	for _, rid := range roles {
		if v, ok := l.roleMenus.Load(rid); ok {
			for k := range v.(map[string]struct{}) {
				allowed[k] = struct{}{}
			}
		}
	}
	// Costruzione nodi filtrati
	nodes := make(map[string]*authcore.MenuNode)
	for id, f := range l.menuCatalog {
		if _, ok := allowed[id]; !ok {
			continue
		}
		// App filter: se filterApp è impostato, includi solo menu con appid == filterApp o senza appid
		if filterApp != "" {
			if f.Menu == nil {
				continue
			}
			if !(f.Menu.AppID == "" || f.Menu.AppID == filterApp) {
				continue
			}
		}
		nodes[id] = &authcore.MenuNode{
			ID:          f.ID,
			Description: f.Description,
			Icon: func() string {
				if f.Menu != nil {
					return f.Menu.Icon
				}
				return ""
			}(),
			Order: func() int {
				if f.Menu != nil {
					return f.Menu.Order
				}
				return 0
			}(),
			IsLeaf: func() bool {
				if f.Menu != nil {
					return f.Menu.IsLeaf
				}
				return false
			}(),
			Endpoint: func() string {
				if f.Menu != nil {
					return f.Menu.Endpoint
				}
				return ""
			}(),
			FunctionParentID: func() string {
				if f.Menu != nil {
					return f.Menu.FunctionParentID
				}
				return ""
			}(),
			Children: nil,
		}
	}
	// Link parent-children solo se parent autorizzato
	roots := make([]*authcore.MenuNode, 0)
	for _, n := range nodes {
		pid := n.FunctionParentID
		if pid == "" {
			roots = append(roots, n)
			continue
		}
		if p, ok := nodes[pid]; ok {
			p.Children = append(p.Children, n)
		} else {
			// parent non autorizzato o non presente → considera root
			roots = append(roots, n)
		}
	}
	return roots
}

// loadMenuCatalog carica tutte le function di tipo menu in memoria.
func (l *AuthorizationLut) loadMenuCatalog(ctx context.Context, collName string) *core.ApplicationError {
	coll := l.ls.GetCollection(collName, "")
	cur, err := coll.Aggregate(ctx, mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"type": "function", "menu": bson.M{"$exists": true}}}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 1, "description": 1, "menu.isleaf": 1, "menu.endpoint": 1, "menu.functionparentid": 1, "menu.icon": 1, "menu.order": 1, "menu.appid": 1}}},
	})
	if err != nil {
		return core.TechnicalErrorWithError(err)
	}
	defer cur.Close(ctx)
	cats := make(map[string]*Function)
	for cur.Next(ctx) {
		var f Function
		if err := cur.Decode(&f); err != nil {
			return core.TechnicalErrorWithError(err)
		}
		cats[f.ID] = &f
	}
	l.menuCatalog = cats
	return nil
}

// loadCapabilityCatalog carica le function di tipo capability (per filtri appid sulle client caps)
func (l *AuthorizationLut) loadCapabilityCatalog(ctx context.Context, collName string) *core.ApplicationError {
	coll := l.ls.GetCollection(collName, "")
	cur, err := coll.Aggregate(ctx, mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"type": "function", "capability": bson.M{"$exists": true}}}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 1, "capability.captype": 1, "capability.appid": 1}}},
	})
	if err != nil {
		return core.TechnicalErrorWithError(err)
	}
	defer cur.Close(ctx)
	cats := make(map[string]*Function)
	for cur.Next(ctx) {
		var f Function
		if err := cur.Decode(&f); err != nil {
			return core.TechnicalErrorWithError(err)
		}
		cats[f.ID] = &f
	}
	l.capCatalog = cats
	return nil
}
