# @pgbeam/crossplane

## 0.3.1

### Patch Changes

- 488eb26: chore(deps): update dependencies to their latest versions

## 0.3.0

### Minor Changes

- 796f4a7: Policy profiles carry `query_mode` (`sql` or `templates`). A profile set to `templates` admits only the project's enabled query templates, and every other rule on the profile still applies to what a template runs. Setting `templates` is refused when the project has no enabled template, and while the organization has a live self-hosted enrollment. On update, an omitted `query_mode` keeps the stored mode. In Terraform, Crossplane and Pulumi, removing `query_mode` from a config plans `sql`, and the plan shows that change.

## 0.2.0

### Minor Changes

- d651ca6: Query templates can be declared as code: `pgbeam_query_template`, the Crossplane `QueryTemplate` and the Pulumi `QueryTemplate`. Removing `description`, `include_sql`, `enabled` or `params` from a config resets it to the API default (empty, false, true, none) rather than keeping the last value. The query template input's `description` now declares its default (empty) in the contract, which is what the API already stored for an omitted one. Nothing restricts a credential to templates yet: the policy profile setting that does arrives in a later release.

### Patch Changes

- 1d9c82e: Log startup failures at Info with the error so a crash-looping provider pod shows why it exited, use production logging unless `--debug` is set, and add `--sync`, `--poll`, `--max-reconcile-rate` and `--leader-election` flags so operators can tune the controller and run more than one replica through a DeploymentRuntimeConfig. Defaults are unchanged.
- c0a650c: Stop the PolicyProfile and Database controllers panicking on nested fields the API or manifest leaves out, and stop Database updates sending an empty sslMode when the manifest omits it.

## 0.1.5

### Patch Changes

- ae1807a: Trim surrounding whitespace from the API key secret so a key stored with a trailing newline no longer breaks every reconcile, reject an empty key with an error naming the secret, and name the webhook signing secret in its read error.
- 7936d07: The CRDs now reject edits to immutable fields with a CEL rule. Previously an edit to a field such as `projectID` or a Replica's `host` was accepted and then silently ignored, or made the provider create a second external resource and orphan the first. Delete and recreate the managed resource to change an immutable field.
- 62e9e8d: Move the OpenTelemetry Go modules to 1.45.0 for GHSA-8wmf-6v46-5gfg. None of them is linked into the provider binary, so behaviour is unchanged.
- 223cb7c: fix(deps): clear govulncheck GO-2026-6809 (crypto/x509) with Go 1.26.10

## 0.1.4

### Patch Changes

- ef1df01: fix(deps): clear govulncheck with Go 1.26.9 and golang.org/x/net 0.60.0

## 0.1.3

### Patch Changes

- 60bb482: Removing an optional field from a manifest now clears it on the server. The up-to-date check used to skip omitted fields on resources updated by PUT (PolicyProfile, WebhookEndpoint, Honeytoken, AnomalyRule, ApprovalRule, SpendLimit, CacheRule), so the old value stayed and the resource reported Synced.
- c1888e5: Release the AnomalyRule and ApprovalRule resources, which were added without a provider version bump.

## 0.1.2

### Patch Changes

- 3342c65: feat(policy): content_scan_mode and content_scan_max_bytes on the policy profile

## 0.1.1

### Patch Changes

- 5d49e15: feat(providers): SelfHostEnrollment resource in all three IaC providers, gateway-resource secret-lifecycle tests, crossplane generator fix for immutable resources with wrapped create responses
