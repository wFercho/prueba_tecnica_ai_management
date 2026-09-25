# Local demo login with database-backed users and sessions

The PDF includes Login in the demonstration path but does not require an identity provider. Use a `users` table with password hashes, and server-side revocable sessions in a `sessions` table; the browser receives only an opaque HttpOnly session cookie. Protect data and analysis endpoints, and let logout revoke the session rather than merely hiding the interface.

## Context

A frontend-only gate would expose credentials and leave the API public. A user file ignored by Git would not exist in an evaluator's clone and would create a second source of truth. An external provider would make the local demo dependent on credentials and network access. The chosen database-backed login is functional without those dependencies, but is **demo-only**, not a claim of production-ready security.

## Consequences

On first `make up`, provision `admin@email.com` with a hash of the demo password `admin` only if the account does not exist. Do not overwrite existing users or sessions on startup or `make seed`. Bind **both the API and the PostgreSQL host port** to `127.0.0.1` by default and document that the known application and database passwords must be replaced before any public deployment. The database can hold multiple users, but account administration has no UI in this MVP; provisioning additional accounts is an administrative operation.
