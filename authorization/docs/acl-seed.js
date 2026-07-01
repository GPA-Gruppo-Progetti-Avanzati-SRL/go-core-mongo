// Script di pre-popolamento per la collection "acl"
// Uso (mongosh):
//   use <your_database>
//   load('docs/acl-seed.js')
//
// Nota: lo script è idempotente. Ogni documento è inserito via upsert su _id.
// Se vuoi ripartire da zero imposta RESET=true qui sotto.

const reset = process.env.RESET || false; // <--- metti true per cancellare tutti i documenti dalla collection

// print reset value
(function(){ print(reset) })();

( function() {
  const coll = db.getCollection('acl');

  if (reset) {
    print('\n[acl-seed] DELETE MANY {}');
    coll.deleteMany({});
  }

  function upsert(doc) {
    coll.replaceOne({ _id: doc._id }, doc, { upsert: true });
  }

  const apps  = [
    {_id: "APP_ROOT",  _et: 'APP', description: "Applicazione principale", path :"/"},
    {_id: "APP_COND", _et: 'APP', description: "Applicazione condizioni", path :"/condizioni"},
    {_id: "APP_CC", _et: 'APP', description: "Applicazione conti correnti", path :"/conti"},
    {_id: "APP_SYSTEM",  _et: 'APP', description: "Applicazione system", path :"/system"},
]
  // --- Funzioni (endpoint/menu/capability) ---
  const capabilities = [

    // api
    { _id: 'API_TOKEN', _et: 'CAPABILITY', category: 'api', description: 'Identità utente corrente', api: { operationid: 'Token' } },
    { _id: 'API_USER_LIST',  _et: 'CAPABILITY' , category: "api", description: 'Elenco utenti', api: { operationid: 'getUsers' } },
    { _id: 'API_CREATE',  _et: 'CAPABILITY' , category: "api", description: 'Crea utente', api: { operationid: 'createUser' } },
    { _id: 'API_LIST',  _et: 'CAPABILITY' , category: "api", description: 'Elenco report', api: { operationid: 'getReports' } },


    // ui
    { _id: 'UI_HOME_ROOT', _et: 'CAPABILITY', description: 'Home', category: "ui", appId :"APP_ROOT", ui: { icon: 'home', endpoint: '/', order: 0  } },
    { _id: 'UI_HOME_CC', _et: 'CAPABILITY', description: 'Home', category: "ui", appId: "APP_CC",  ui: { icon: 'home', endpoint: '/', order: 0 } },
    { _id: 'UI_HOME_COND', _et: 'CAPABILITY', description: 'Home', category: "ui", appId: "APP_COND",  ui: { icon: 'home', endpoint: '/', order: 0 } },
    { _id: 'UI_HOME_SYSTEM', _et: 'CAPABILITY', description: 'Home', category: "ui", appId: "APP_SYSTEM",  ui: { icon: 'home', endpoint: '/', order: 0 } },
    { _id: 'UI_USERS_NEW',  _et: 'CAPABILITY', description: 'Menu Gestione Utenti',  category: 'ui', ui: { endpoint: '/users/new', icon: 'users', order: 10 } },
    { _id: 'UI_USERS_LIST', _et: 'CAPABILITY', category: 'ui', description: 'Elenco utenti (menu)', ui: { endpoint: '/users', icon: 'list', order: 11 } },
    { _id: 'UI_REPORTS', _et: 'CAPABILITY', category: 'ui', description: 'Menu Reportistica', ui: { icon: 'chart', order: 20 , endpoint: '/reports' } },

    // actions
    { _id: 'ACT_UI_EXPORT', _et: 'CAPABILITY',  category : "action_ui", description: 'Esporta dati' ,appId :"APP_HOME" },
    { _id: 'ACT_API_SERVER', _et: 'CAPABILITY', category : "action_api" , description: 'Cap server' },

  ];

  // --- Function Groups ---
  const capabilitygroups = [
    // functiongroup comune per operazioni base
    {
      _id: 'CG_COMMON',
      _et: 'CAPABILITYGROUP',
      description: 'Funzioni comuni',
      capabilities: ['API_TOKEN','UI_HOME_ROOT'],
    },
    {
      _id: 'CG_USERS',
      _et: 'CAPABILITYGROUP',
      description: 'Gestione utenti',
      capabilities: ['API_USER_LIST', 'API_CREATE', 'UI_USERS_NEW', 'UI_USERS_LIST', 'ACT_UI_EXPORT'],
    },
    {
      _id: 'CG_REPORTS',
      _et: 'CAPABILITYGROUP',
      description: 'Reportistica',
      capabilities: ['API_LIST', 'UI_REPORTS', 'ACT_API_SERVER'],
    },
  ];

  // --- Ruoli ---
  const roles = [
    {
      _id: 'ROLE_ADMIN',
      _et: 'ROLE',
      description: 'Amministratore',
      capability_groups: ['CG_COMMON', 'CG_USERS', 'CG_REPORTS'],
      capabilities: ['ACT_UI_EXPORT', 'ACT_API_SERVER'],
    },
    {
      _id: 'ROLE_USER',
      _et: 'ROLE',
      description: 'Utente base',
      capability_groups: ['CG_COMMON', 'CG_USERS'],
    },
    {
      _id: 'ROLE_REPORT',
      _et: 'ROLE',
      description: 'Utente report',
      capability_groups: ['CG_COMMON', 'CG_REPORTS'],
    },
  ];

  print('\n[acl-seed] Upsert capabilities ...');
  capabilities.forEach(upsert);

  print('[acl-seed] Upsert capabilitygroups ...');
  capabilitygroups.forEach(upsert);

  print('[acl-seed] Upsert roles ...');
  roles.forEach(upsert);


  print('[acl-seed] Upsert apps ...');
  apps.forEach(upsert);

  print('[acl-seed] Done.');
})();
