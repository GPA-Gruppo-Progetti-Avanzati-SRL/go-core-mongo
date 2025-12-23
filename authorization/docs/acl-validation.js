// Validator for the "acl" collection to ensure that documents of type
// function contain exactly one of the nodes: endpoint, menu, capability.
// This version avoids $jsonSchema if/then (not supported by Mongo validators)
// and uses a classic query validator compatible with all supported versions.

(function () {
  const coll = db.getCollection('acl');

  // Rule:
  // - if type != 'function' -> pass
  // - else exactly one of the three fields must exist
  const validator = {
    $or: [
      // Non-function docs: always pass
      { type: { $ne: 'function' } },

      // Case: endpoint only, and operationid required (non-empty string)
      { $and: [
        { endpoint: { $exists: true } },
        { menu: { $exists: false } },
        { capability: { $exists: false } },
        { 'endpoint.operationid': { $type: 'string', $ne: '' } },
      ]},

      // Case: menu only (no extra required fields enforced here)
      { $and: [
        { menu: { $exists: true } },
        { endpoint: { $exists: false } },
        { capability: { $exists: false } },
      ]},

      // Case: capability only, captype required and valid
      { $and: [
        { capability: { $exists: true } },
        { endpoint: { $exists: false } },
        { menu: { $exists: false } },
        { 'capability.captype': { $in: ['client', 'server'] } },
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
