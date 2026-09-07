# PgBeam Crossplane Provider

Crossplane provider for [PgBeam](https://pgbeam.com) — manage your globally distributed PostgreSQL proxy infrastructure using Kubernetes custom resources.

## Install

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-pgbeam
spec:
  package: ghcr.io/sferarc/provider-pgbeam:latest
```

## Usage

A project is created together with its primary database in one call, so the `database` object is required and immutable, as is `orgID`. A project has no `region`: by default (`residency: any`) PgBeam serves it from every metro and routes each client to the nearest one. Set `residency` to `us` or `eu` to require the serving metro to be in that jurisdiction. Where a connection pool lives is a per-database choice, via `poolRegion`.

```yaml
apiVersion: pgbeam.io/v1alpha1
kind: Project
metadata:
  name: my-project
spec:
  forProvider:
    orgID: org_123
    name: my-project
    database:
      host: your-db-host.example.com
      port: 5432
      name: mydb
      username: dbuser
      sslMode: require
      passwordSecretRef:
        name: db-credentials
        namespace: default
        key: password
```

To attach more databases later (a read replica, say), use the standalone `Database` resource with its own `projectID`. On both, `name` is the PostgreSQL database name on your server.

```yaml
apiVersion: pgbeam.io/v1alpha1
kind: Database
metadata:
  name: analytics
spec:
  forProvider:
    projectID: prj_123
    host: replica-host.example.com
    port: 5432
    name: mydb
    username: dbuser
    role: replica
    passwordSecretRef:
      name: db-credentials
      namespace: default
      key: password
```

## Resources

| Kind | API Version | Description |
| --- | --- | --- |
| `Project` | `pgbeam.io/v1alpha1` | PgBeam project |
| `Database` | `pgbeam.io/v1alpha1` | PostgreSQL database connection |
| `Replica` | `pgbeam.io/v1alpha1` | Read replica configuration |
| `CustomDomain` | `pgbeam.io/v1alpha1` | Custom domain for connection strings |
| `CacheRule` | `pgbeam.io/v1alpha1` | Query caching rule |
| `SpendLimit` | `pgbeam.io/v1alpha1` | Budget controls |
| `AgentCredential` | `pgbeam.io/v1alpha1` | Scoped agent credential |
| `PolicyProfile` | `pgbeam.io/v1alpha1` | Policy profile (access mode, allowlists, masking, budgets) |
| `WebhookEndpoint` | `pgbeam.io/v1alpha1` | Event delivery endpoint |

## Agent gateway

The agent gateway issues scoped, policy-enforced credentials for AI agents and delivers audit/anomaly events to webhook endpoints.

```yaml
apiVersion: pgbeam.io/v1alpha1
kind: WebhookEndpoint
metadata:
  name: audit
spec:
  forProvider:
    projectID: prj_123
    url: https://example.com/hooks/pgbeam
    format: json
    eventTypes: [blocked, anomaly, approval]
    enabled: true
    secretSecretRef: # write-only signing secret
      name: webhook-secret
      namespace: default
      key: secret

---
apiVersion: pgbeam.io/v1alpha1
kind: AgentCredential
metadata:
  name: analytics
spec:
  forProvider:
    projectID: prj_123
    policyProfileID: pol_123
    name: Claude Code (analytics)
    principalType: agent
  # One-time secrets (connectionString, mcpToken) are published to this secret.
  writeConnectionSecretToRef:
    name: analytics-agent-secrets
    namespace: default
```

> **Agent credential secrets caveat.** The one-time `connection_string` and `mcp_token` are returned only at creation and are published to the `writeConnectionSecretToRef` Secret (keys `connectionString`, `mcpToken`) rather than stored in the resource status. The non-secret `mcpURL` is exposed in `status.atProvider`. To rotate, delete and recreate the resource.

Manage policies as code with the `PolicyProfile` resource:

```yaml
apiVersion: pgbeam.io/v1alpha1
kind: PolicyProfile
metadata:
  name: read-only
spec:
  forProvider:
    projectID: prj_123
    name: read-only
    accessMode: read_only
```

A `PolicyProfile` publishes its ID as `status.atProvider.id`. Supply that value wherever a profile is required: `policyProfileID` on an `AgentCredential` (see the example above), or `defaultPolicyProfileID` on a `Project` to enforce a profile on passthrough/human connections. Keeping the profile here puts the most security-sensitive primitive under Crossplane drift reconciliation.

## Authentication

Create a Kubernetes secret with your PgBeam API key and point `apiKeySecretRef` at it:

```yaml
apiVersion: pgbeam.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  apiKeySecretRef:
    name: pgbeam-credentials
    namespace: crossplane-system
    key: api-key
```

Managed resources use the `ProviderConfig` named `default` unless they set their own `providerConfigRef`. The API base URL defaults to `https://api.pgbeam.com` and can be overridden with `spec.baseUrl`.

## Documentation

Full usage guide at [pgbeam.com/docs/crossplane](https://pgbeam.com/docs/crossplane).

## Contributing

Issues and pull requests are welcome here. An issue is the right place to start for a bug, a wrong doc, or a missing capability; say what you ran, what happened, what you expected, and which version you were on.

To build and test it locally:

```bash
go build ./...
go test ./...
```

Do not open a public issue for a suspected security vulnerability. Email security@pgbeam.com, or report it privately from this repository's Security tab.

## License

Apache 2.0 — see [LICENSE](LICENSE).
