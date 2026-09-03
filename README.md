# PgBeam Crossplane Provider

Crossplane provider for [PgBeam](https://pgbeam.com) — manage your globally
distributed PostgreSQL proxy infrastructure using Kubernetes custom resources.

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

```yaml
apiVersion: pgbeam.io/v1alpha1
kind: Project
metadata:
  name: my-project
spec:
  forProvider:
    name: my-project
    orgId: org_123
    region: us-east-1

---
apiVersion: pgbeam.io/v1alpha1
kind: Database
metadata:
  name: primary
spec:
  forProvider:
    projectIdRef:
      name: my-project
    name: primary
    host: your-db-host.example.com
    port: 5432
    database: mydb
    username: dbuser
    passwordSecretRef:
      name: db-credentials
      namespace: default
      key: password
```

## Resources

| Kind              | API Version          | Description                          |
| ----------------- | -------------------- | ------------------------------------ |
| `Project`         | `pgbeam.io/v1alpha1` | PgBeam project                       |
| `Database`        | `pgbeam.io/v1alpha1` | PostgreSQL database connection       |
| `Replica`         | `pgbeam.io/v1alpha1` | Read replica configuration           |
| `CustomDomain`    | `pgbeam.io/v1alpha1` | Custom domain for connection strings |
| `CacheRule`       | `pgbeam.io/v1alpha1` | Query caching rule                   |
| `SpendLimit`      | `pgbeam.io/v1alpha1` | Budget controls                      |
| `AgentCredential` | `pgbeam.io/v1alpha1` | Scoped agent credential              |
| `WebhookEndpoint` | `pgbeam.io/v1alpha1` | Event delivery endpoint              |

## Agent gateway

The agent gateway issues scoped,
policy-enforced credentials for AI agents and delivers audit/anomaly events to
webhook endpoints.

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

> **Agent credential secrets caveat.** The one-time `connection_string` and
> `mcp_token` are returned only at creation and are published to the
> `writeConnectionSecretToRef` Secret (keys `connectionString`, `mcpToken`)
> rather than stored in the resource status. The non-secret `mcpUrl` is exposed
> in `status.atProvider`. To rotate, delete and recreate the resource.

> **Policy profiles are not yet managed as code.** `policyProfileID` (above, and
> `defaultPolicyProfileID` on a `Project`) is the ID of a policy profile that
> must be created out of band with `pgbeam policies create` or the dashboard —
> there is no `PolicyProfile` managed resource yet. The policy itself, the most
> security-sensitive primitive, therefore lives outside your reviewed GitOps flow
> and is invisible to Crossplane drift reconciliation.

## Authentication

Create a Kubernetes secret with your PgBeam API token and reference it in a
`ProviderConfig`:

```yaml
apiVersion: pgbeam.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      name: pgbeam-credentials
      namespace: crossplane-system
      key: api-token
```

## Documentation

Full usage guide at
[pgbeam.com/docs/crossplane](https://pgbeam.com/docs/crossplane).

## Contributing

Issues and pull requests are welcome here. An issue is the right place to start
for a bug, a wrong doc, or a missing capability; say what you ran, what
happened, what you expected, and which version you were on.

To build and test it locally:

```bash
go build ./...
go test ./...
```

Do not open a public issue for a suspected security vulnerability. Email
security@pgbeam.com, or report it privately from this repository's Security
tab.

## License

Apache 2.0 — see [LICENSE](LICENSE).
