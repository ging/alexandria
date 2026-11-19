-- Crea bases de datos aisladas para Keycloak, el Vocabulary Hub (server) y el sidecar (client).
SELECT 'CREATE DATABASE keycloak'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'keycloak')\gexec

SELECT 'CREATE DATABASE alexandria_hub'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'alexandria_hub')\gexec

SELECT 'CREATE DATABASE alexandria_client'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'alexandria_client')\gexec
