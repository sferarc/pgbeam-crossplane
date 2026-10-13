# @pgbeam/crossplane

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
