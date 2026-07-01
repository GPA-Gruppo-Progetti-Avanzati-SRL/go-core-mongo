export mongoServer=mongodb+srv://scatolotto_usr:scatolotto_pwd.123@mongo.gpagroup.it/scatolotto?tls=false
export RESET=true

mongosh $mongoServer --file acl-seed.js
mongosh $mongoServer --file acl-validation.js
mongosh $mongoServer --file acl-indexes.js
