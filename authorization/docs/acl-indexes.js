// Indici suggeriti per la collection "acl"
// Eseguire questi comandi nella shell MongoDB sul database scelto

// Indice per filtrare rapidamente per tipo (discriminatore)
db.acl.createIndex({ _et: 1 })

// Indice per ricerche su membership dei capability group all'interno dei ruoli
db.acl.createIndex({ _et: 1, capability_groups: 1 })

// Indice per ricerche su membership delle capability all'interno dei gruppi o ruoli
db.acl.createIndex({ _et: 1, capabilities: 1 })

// Indici per i documenti di tipo CAPABILITY (schema nested)

// API: indicizzazione operationid per il RoleMatcher
db.acl.createIndex(
  { _et: 1, 'api.operationid': 1 },
  { partialFilterExpression: { _et: 'CAPABILITY', 'api.operationid': { $exists: true } } }
)

// UI: indicizzazione appId per il filtraggio menu
db.acl.createIndex(
  { _et: 1, appId: 1 },
  { partialFilterExpression: { _et: 'CAPABILITY', appId: { $exists: true } } }
)

// Categoria: per facilitare il raggruppamento nell'aggregazione della LUT
db.acl.createIndex(
  { _et: 1, category: 1 },
  { partialFilterExpression: { _et: 'CAPABILITY' } }
)

