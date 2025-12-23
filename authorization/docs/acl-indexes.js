// Indici suggeriti per la collection "acl"
// Eseguire questi comandi nella shell MongoDB sul database scelto

// Indice per filtrare rapidamente per tipo
db.acl.createIndex({ type: 1 })

// Indice per ricerche su membership dei function group all'interno dei ruoli
db.acl.createIndex({ type: 1, functiongroups: 1 })

// Indice per ricerche su membership delle function all'interno dei function group
db.acl.createIndex({ type: 1, functions: 1 })

// Indici per i documenti di tipo function (schema nested, senza "kind")
// Endpoint: indicizzazione con Partial Filter Expression per i soli documenti con endpoint
db.acl.createIndex(
  { type: 1, 'endpoint.operationid': 1 },
  { partialFilterExpression: { type: 'function', 'endpoint.operationid': { $exists: true } } }
)
// Capabilities
db.acl.createIndex(
  { type: 1, 'capability.captype': 1 },
  { partialFilterExpression: { type: 'function', 'capability.captype': { $exists: true } } }
)
db.acl.createIndex(
  { type: 1, 'capability.appid': 1 },
  { partialFilterExpression: { type: 'function', 'capability.appid': { $exists: true } } }
)
// Menu
db.acl.createIndex(
  { type: 1, 'menu.appid': 1 },
  { partialFilterExpression: { type: 'function', 'menu.appid': { $exists: true } } }
)

// Indici aggiuntivi per la gestione del menu ad albero (nested)
db.acl.createIndex(
  { type: 1, 'menu.isleaf': 1 },
  { partialFilterExpression: { type: 'function', 'menu.isleaf': { $exists: true } } }
)
db.acl.createIndex(
  { type: 1, 'menu.functionparentid': 1 },
  { partialFilterExpression: { type: 'function', 'menu.functionparentid': { $exists: true } } }
)
