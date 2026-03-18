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

| Kind           | API Version          | Description                          |
| -------------- | -------------------- | ------------------------------------ |
| `Project`      | `pgbeam.io/v1alpha1` | PgBeam project                       |
| `Database`     | `pgbeam.io/v1alpha1` | PostgreSQL database connection       |
| `Replica`      | `pgbeam.io/v1alpha1` | Read replica configuration           |
| `CustomDomain` | `pgbeam.io/v1alpha1` | Custom domain for connection strings |
| `CacheRule`    | `pgbeam.io/v1alpha1` | Query caching rule                   |
| `SpendLimit`   | `pgbeam.io/v1alpha1` | Budget controls                      |

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
[docs.pgbeam.com/crossplane](https://docs.pgbeam.com/crossplane).

## License

Apache 2.0 — see [LICENSE](LICENSE).
