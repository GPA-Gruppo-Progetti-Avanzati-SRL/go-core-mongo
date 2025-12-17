// Indici suggeriti per la collection "acl"
// Eseguire questi comandi nella shell MongoDB sul database scelto

// Indice per filtrare rapidamente per tipo
db.acl.createIndex({ type: 1 })

// Indice per ricerche su membership dei function group all'interno dei ruoli
db.acl.createIndex({ type: 1, functiongroups: 1 })

// Indice per ricerche su membership delle function all'interno dei function group
db.acl.createIndex({ type: 1, functions: 1 })
