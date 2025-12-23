// Script di pre-popolamento per la collection "acl"
// Uso (mongosh):
//   use <your_database>
//   load('docs/acl-seed.js')
//
// Nota: lo script è idempotente. Ogni documento è inserito via upsert su _id.
// Se vuoi ripartire da zero imposta RESET=true qui sotto.

(function () {
  const coll = db.getCollection('acl');
  const RESET = true; // <--- metti true per cancellare tutti i documenti dalla collection

  if (RESET) {
    print('\n[acl-seed] DELETE MANY {}');
    coll.deleteMany({});
  }

  function upsert(doc) {
    coll.replaceOne({ _id: doc._id }, doc, { upsert: true });
  }

  // --- Funzioni (endpoint/menu/capability) ---
  const functions = [

    // endpoint comuni a tutti i ruoli (nested)
    { _id: 'FUNC_WHOAMI', type: 'function', description: 'Identità utente corrente', endpoint: { operationid: 'whoami' } },
    { _id: 'FUNC_MENU', type: 'function', description: 'Costruzione menu', endpoint: { operationid: 'menu' } },

    // endpoint (nested)
    { _id: 'FUNC_USER_LIST', type: 'function', description: 'Elenco utenti', endpoint: { operationid: 'getUsers' } },
    { _id: 'FUNC_USER_CREATE', type: 'function', description: 'Crea utente', endpoint: { operationid: 'createUser' } },
    { _id: 'FUNC_REPORT_LIST', type: 'function', description: 'Elenco report', endpoint: { operationid: 'getReports' } },


    // menu (nested, con icone e ordinamento)
    { _id: 'MENU_HOME', type: 'function', description: 'Home', menu: { icon: 'home', isleaf: true, endpoint: '/', order: 0 } },
    { _id: 'MENU_USERS', type: 'function', description: 'Menu Gestione Utenti', menu: { icon: 'users', order: 10 } },
    { _id: 'MENU_USERS_LIST', type: 'function', description: 'Elenco utenti (menu)', menu: { isleaf: true, endpoint: '/crud', functionparentid: 'MENU_USERS', icon: 'list', order: 11 } },
    { _id: 'MENU_REPORTS', type: 'function', description: 'Menu Reportistica', menu: { isleaf: true, icon: 'chart', order: 20 , endpoint: '/reports', appid: 'APP_A' } },

    // capability (nested)
    { _id: 'CAP_EXPORT', type: 'function', description: 'Esporta dati', capability: { captype: 'client', appid: 'APP_A' } },
    { _id: 'CAP_SERVER', type: 'function', description: 'Cap server', capability: { captype: 'server' } },

  ];

  // --- Function Groups ---
  const functiongroups = [
    // functiongroup comune per operazioni base
    {
      _id: 'FG_COMMON',
      type: 'functiongroup',
      description: 'Funzioni comuni',
      functions: ['FUNC_WHOAMI', 'FUNC_MENU','MENU_HOME'],
    },
    {
      _id: 'FG_USERS',
      type: 'functiongroup',
      description: 'Gestione utenti',
      // riferimenti a functions per ID
      functions: ['FUNC_USER_LIST', 'FUNC_USER_CREATE', 'MENU_USERS', 'MENU_USERS_LIST', 'CAP_EXPORT'],
    },
    {
      _id: 'FG_REPORTS',
      type: 'functiongroup',
      description: 'Reportistica',
      functions: ['FUNC_REPORT_LIST', 'MENU_REPORTS', 'CAP_SERVER'],
    },
  ];

  // --- Ruoli ---
  const roles = [
    {
      _id: 'ROLE_ADMIN',
      type: 'role',
      description: 'Amministratore',
      // riferimenti a functiongroups per ID
      functiongroups: ['FG_COMMON', 'FG_USERS', 'FG_REPORTS'],
    },
    {
      _id: 'ROLE_USER',
      type: 'role',
      description: 'Utente base',
      functiongroups: ['FG_COMMON', 'FG_USERS'],
    },
    {
      _id: 'ROLE_REPORT',
      type: 'role',
      description: 'Utente report',
      functiongroups: ['FG_COMMON', 'FG_REPORTS'],
    },
  ];

  print('\n[acl-seed] Upsert functions ...');
  functions.forEach(upsert);

  print('[acl-seed] Upsert functiongroups ...');
  functiongroups.forEach(upsert);

  print('[acl-seed] Upsert roles ...');
  roles.forEach(upsert);

  print('[acl-seed] Done.');
})();
