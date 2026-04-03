// Optional MongoDB initialization for local development (docker-compose.dev.yml).
// Extend with collections or users when the app requires them.
db = db.getSiblingDB('chaoslabs');
