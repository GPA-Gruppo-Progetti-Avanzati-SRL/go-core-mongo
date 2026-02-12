// Validator for the "acl" collection to ensure that documents of type
// function contain exactly one of the nodes: endpoint, menu, capability.
// This version avoids $jsonSchema if/then (not supported by Mongo validators)
// and uses a classic query validator compatible with all supported versions.

(function () {
  const coll = db.getCollection('acl');

  // Rule:
  // - if _et != 'CAPABILITY' -> pass
  // - else exactly one of the fields 'api' or 'ui' (or none for generic capabilities)
  const validator = {
    $or: [
      // Non-capability docs: always pass
      { _et: { $ne: 'CAPABILITY' } },

      // Case: api capability
      { $and: [
        { api: { $exists: true } },
        { ui: { $exists: false } },
        { 'api.operationid': { $type: 'string', $ne: '' } },
      ]},

      // Case: ui capability
      { $and: [
        { ui: { $exists: true } },
        { api: { $exists: false } },
      ]},

      // Case: generic capability (no api/ui, just the document)
      { $and: [
        { ui: { $exists: false } },
        { api: { $exists: false } },
      ]},
    ],
  };

  // Applica il validator alla collection
  db.runCommand({
    collMod: 'acl',
    validator: validator,
    validationLevel: 'moderate',
    validationAction: 'error',
  });

  print('[acl-validation] Validator applied.');
})();
