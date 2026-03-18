package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ============================================================================
// ProviderConfig
// ============================================================================

// ProviderConfigSpec defines the desired state of the provider configuration.
type ProviderConfigSpec struct {
	// APIKeySecretRef is a reference to a Secret containing the PgBeam API key.
	// The key must be stored under the "apiKey" data field.
	// +kubebuilder:validation:Required
	APIKeySecretRef xpv1.SecretKeySelector `json:"apiKeySecretRef"`

	// BaseURL is the PgBeam API base URL. Defaults to https://api.pgbeam.com.
	// +optional
	// +kubebuilder:default="https://api.pgbeam.com"
	BaseURL string `json:"baseUrl,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=crossplane
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ProviderConfig configures how the provider connects to the PgBeam API.
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// ProviderConfigUsage indicates a usage of a ProviderConfig.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=crossplane
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv1.ProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}

// ============================================================================
// Project
// ============================================================================

// ProjectDatabaseSpec defines the primary database created with a project.
type ProjectDatabaseSpec struct {
	// Host is the upstream PostgreSQL host.
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Port is the upstream PostgreSQL port (1-65535).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	Port int `json:"port"`

	// Name is the PostgreSQL database name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Username is the PostgreSQL username.
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// PasswordSecretRef is a reference to a Secret containing the database password.
	// +kubebuilder:validation:Required
	PasswordSecretRef xpv1.SecretKeySelector `json:"passwordSecretRef"`

	// SSLMode is the SSL connection mode.
	// +optional
	// +kubebuilder:validation:Enum=disable;allow;prefer;require;verify-ca;verify-full
	SSLMode string `json:"sslMode,omitempty"`

	// Role is the database role: primary or replica.
	// +optional
	// +kubebuilder:validation:Enum=primary;replica
	Role string `json:"role,omitempty"`

	// PoolRegion is the region for the connection pool (e.g. "us-east-1").
	// +optional
	PoolRegion *string `json:"poolRegion,omitempty"`

	// CacheConfig configures query caching for this database.
	// +optional
	CacheConfig *CacheConfigSpec `json:"cacheConfig,omitempty"`

	// PoolConfig configures connection pooling for this database.
	// +optional
	PoolConfig *PoolConfigSpec `json:"poolConfig,omitempty"`
}

// CacheConfigSpec configures query caching for a database.
type CacheConfigSpec struct {
	// Enabled enables or disables query caching.
	Enabled bool `json:"enabled"`

	// TTLSeconds is the cache time-to-live in seconds.
	// +optional
	TTLSeconds int `json:"ttlSeconds,omitempty"`

	// MaxEntries is the maximum number of cached entries.
	// +optional
	MaxEntries int `json:"maxEntries,omitempty"`

	// SWRSeconds is the stale-while-revalidate window in seconds.
	// +optional
	SWRSeconds int `json:"swrSeconds,omitempty"`
}

// PoolConfigSpec configures connection pooling for a database.
type PoolConfigSpec struct {
	// PoolSize is the maximum pool size.
	// +optional
	PoolSize int `json:"poolSize,omitempty"`

	// MinPoolSize is the minimum pool size.
	// +optional
	MinPoolSize int `json:"minPoolSize,omitempty"`

	// PoolMode is the pooling mode (transaction or session).
	// +optional
	// +kubebuilder:validation:Enum=transaction;session
	PoolMode string `json:"poolMode,omitempty"`
}

// ProjectForProvider defines the desired state of a Project.
type ProjectForProvider struct {
	// OrgID is the organization that owns this project.
	// +kubebuilder:validation:Required
	// +immutable
	OrgID string `json:"orgId"`

	// Name is the human-readable project name (1-100 characters).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	Name string `json:"name"`

	// Description is an optional project description (up to 500 characters).
	// +optional
	// +kubebuilder:validation:MaxLength=500
	Description *string `json:"description,omitempty"`

	// Tags are optional user-defined labels (max 10, each up to 50 chars).
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Cloud is the cloud provider. Defaults to "aws".
	// +optional
	// +immutable
	// +kubebuilder:validation:Enum=aws;azure;gcp
	// +kubebuilder:default=aws
	Cloud string `json:"cloud,omitempty"`

	// QueriesPerSecond is the max sustained QPS (0 = unlimited).
	// +optional
	QueriesPerSecond *int32 `json:"queriesPerSecond,omitempty"`

	// BurstSize is the burst allowance above sustained QPS.
	// +optional
	BurstSize *int32 `json:"burstSize,omitempty"`

	// MaxConnections is the max concurrent connections (0 = unlimited).
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`

	// Database is the primary database configuration, created atomically.
	// +kubebuilder:validation:Required
	Database ProjectDatabaseSpec `json:"database"`
}

// ProjectAtProvider defines the observed state of a Project.
type ProjectAtProvider struct {
	// ID is the PgBeam project ID.
	ID string `json:"id,omitempty"`

	// ProxyHost is the PgBeam proxy hostname for this project.
	ProxyHost string `json:"proxyHost,omitempty"`

	// Status is the project status: active, suspended, or deleted.
	Status string `json:"status,omitempty"`

	// DatabaseCount is the number of databases attached.
	DatabaseCount int `json:"databaseCount,omitempty"`

	// ActiveConnections is the current active connection count.
	ActiveConnections int `json:"activeConnections,omitempty"`

	// PrimaryDatabaseID is the ID of the primary database created with the project.
	PrimaryDatabaseID string `json:"primaryDatabaseId,omitempty"`

	// CreatedAt is the ISO 8601 creation timestamp.
	CreatedAt string `json:"createdAt,omitempty"`

	// UpdatedAt is the ISO 8601 last-update timestamp.
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// ProjectSpec defines the desired state of a Project.
type ProjectSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ProjectForProvider `json:"forProvider"`
}

// ProjectStatus defines the observed state of a Project.
type ProjectStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ProjectAtProvider `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane;managed;pgbeam
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Project is a managed resource that represents a PgBeam Project.
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Projects.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

// ============================================================================
// Database
// ============================================================================

// DatabaseForProvider defines the desired state of a Database.
type DatabaseForProvider struct {
	// ProjectID is the parent project ID.
	// +kubebuilder:validation:Required
	// +immutable
	ProjectID string `json:"projectId"`

	// Host is the upstream PostgreSQL host.
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Port is the upstream PostgreSQL port (1-65535).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	Port int `json:"port"`

	// Name is the PostgreSQL database name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Username is the PostgreSQL username.
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// PasswordSecretRef is a reference to a Secret containing the database password.
	// +kubebuilder:validation:Required
	PasswordSecretRef xpv1.SecretKeySelector `json:"passwordSecretRef"`

	// SSLMode is the SSL connection mode.
	// +optional
	// +kubebuilder:validation:Enum=disable;allow;prefer;require;verify-ca;verify-full
	SSLMode string `json:"sslMode,omitempty"`

	// Role is the database role: primary or replica.
	// +optional
	// +kubebuilder:validation:Enum=primary;replica
	Role string `json:"role,omitempty"`

	// PoolRegion is the region for the connection pool.
	// +optional
	PoolRegion *string `json:"poolRegion,omitempty"`

	// CacheConfig configures query caching for this database.
	// +optional
	CacheConfig *CacheConfigSpec `json:"cacheConfig,omitempty"`

	// PoolConfig configures connection pooling for this database.
	// +optional
	PoolConfig *PoolConfigSpec `json:"poolConfig,omitempty"`
}

// DatabaseAtProvider defines the observed state of a Database.
type DatabaseAtProvider struct {
	// ID is the PgBeam database ID.
	ID string `json:"id,omitempty"`

	// ConnectionString is the upstream connection string.
	ConnectionString string `json:"connectionString,omitempty"`

	// CreatedAt is the ISO 8601 creation timestamp.
	CreatedAt string `json:"createdAt,omitempty"`

	// UpdatedAt is the ISO 8601 last-update timestamp.
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// DatabaseSpec defines the desired state of a Database.
type DatabaseSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       DatabaseForProvider `json:"forProvider"`
}

// DatabaseStatus defines the observed state of a Database.
type DatabaseStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          DatabaseAtProvider `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane;managed;pgbeam
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Database is a managed resource that represents a PgBeam Database.
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec"`
	Status DatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Databases.
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

// ============================================================================
// Replica
// ============================================================================

// ReplicaForProvider defines the desired state of a Replica.
type ReplicaForProvider struct {
	// DatabaseID is the parent database ID.
	// +kubebuilder:validation:Required
	// +immutable
	DatabaseID string `json:"databaseId"`

	// Host is the replica PostgreSQL host.
	// +kubebuilder:validation:Required
	// +immutable
	Host string `json:"host"`

	// Port is the replica PostgreSQL port (1-65535).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	// +immutable
	Port int `json:"port"`

	// SSLMode is the SSL connection mode.
	// +optional
	// +immutable
	// +kubebuilder:validation:Enum=disable;allow;prefer;require;verify-ca;verify-full
	SSLMode string `json:"sslMode,omitempty"`
}

// ReplicaAtProvider defines the observed state of a Replica.
type ReplicaAtProvider struct {
	// ID is the PgBeam replica ID.
	ID string `json:"id,omitempty"`

	// CreatedAt is the ISO 8601 creation timestamp.
	CreatedAt string `json:"createdAt,omitempty"`

	// UpdatedAt is the ISO 8601 last-update timestamp.
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// ReplicaSpec defines the desired state of a Replica.
type ReplicaSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ReplicaForProvider `json:"forProvider"`
}

// ReplicaStatus defines the observed state of a Replica.
type ReplicaStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ReplicaAtProvider `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane;managed;pgbeam
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Replica is a managed resource that represents a PgBeam Replica.
// All fields are immutable; any change triggers recreation.
type Replica struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicaSpec   `json:"spec"`
	Status ReplicaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReplicaList contains a list of Replicas.
type ReplicaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Replica `json:"items"`
}

// ============================================================================
// CustomDomain
// ============================================================================

// CustomDomainForProvider defines the desired state of a CustomDomain.
type CustomDomainForProvider struct {
	// ProjectID is the parent project ID.
	// +kubebuilder:validation:Required
	// +immutable
	ProjectID string `json:"projectId"`

	// Domain is the custom domain name (e.g. "db.example.com").
	// +kubebuilder:validation:Required
	// +immutable
	Domain string `json:"domain"`
}

// DNSInstructionsObservation contains DNS records needed for domain verification.
type DNSInstructionsObservation struct {
	// CNAMEHost is the CNAME record host.
	CNAMEHost string `json:"cnameHost,omitempty"`

	// CNAMETarget is the CNAME record target.
	CNAMETarget string `json:"cnameTarget,omitempty"`

	// TXTHost is the TXT record host.
	TXTHost string `json:"txtHost,omitempty"`

	// TXTValue is the TXT record value.
	TXTValue string `json:"txtValue,omitempty"`

	// ACMECNAMEHost is the ACME CNAME record host.
	ACMECNAMEHost string `json:"acmeCnameHost,omitempty"`

	// ACMECNAMETarget is the ACME CNAME record target.
	ACMECNAMETarget string `json:"acmeCnameTarget,omitempty"`
}

// CustomDomainAtProvider defines the observed state of a CustomDomain.
type CustomDomainAtProvider struct {
	// ID is the PgBeam custom domain ID.
	ID string `json:"id,omitempty"`

	// Verified indicates whether the domain has been verified.
	Verified bool `json:"verified,omitempty"`

	// VerifiedAt is the ISO 8601 timestamp of verification.
	VerifiedAt string `json:"verifiedAt,omitempty"`

	// TLSCertExpiry is the ISO 8601 timestamp of TLS certificate expiration.
	TLSCertExpiry string `json:"tlsCertExpiry,omitempty"`

	// DNSVerificationToken is the token for DNS verification.
	DNSVerificationToken string `json:"dnsVerificationToken,omitempty"`

	// DNSInstructions contains the DNS records needed for verification.
	DNSInstructions *DNSInstructionsObservation `json:"dnsInstructions,omitempty"`

	// CreatedAt is the ISO 8601 creation timestamp.
	CreatedAt string `json:"createdAt,omitempty"`

	// UpdatedAt is the ISO 8601 last-update timestamp.
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// CustomDomainSpec defines the desired state of a CustomDomain.
type CustomDomainSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       CustomDomainForProvider `json:"forProvider"`
}

// CustomDomainStatus defines the observed state of a CustomDomain.
type CustomDomainStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          CustomDomainAtProvider `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane;managed;pgbeam
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// CustomDomain is a managed resource that represents a PgBeam Custom Domain.
// All fields are immutable; any change triggers recreation.
type CustomDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomDomainSpec   `json:"spec"`
	Status CustomDomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CustomDomainList contains a list of CustomDomains.
type CustomDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomDomain `json:"items"`
}

// ============================================================================
// CacheRule
// ============================================================================

// CacheRuleForProvider defines the desired state of a CacheRule.
type CacheRuleForProvider struct {
	// ProjectID is the parent project ID.
	// +kubebuilder:validation:Required
	// +immutable
	ProjectID string `json:"projectId"`

	// DatabaseID is the parent database ID.
	// +kubebuilder:validation:Required
	// +immutable
	DatabaseID string `json:"databaseId"`

	// QueryHash is the xxhash64 hex of the normalized SQL (16-char hex string).
	// +kubebuilder:validation:Required
	// +immutable
	QueryHash string `json:"queryHash"`

	// CacheEnabled enables or disables caching for this query shape.
	// +kubebuilder:validation:Required
	CacheEnabled bool `json:"cacheEnabled"`

	// CacheTTLSeconds is the TTL override in seconds (0-86400).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=86400
	CacheTTLSeconds *int `json:"cacheTtlSeconds,omitempty"`

	// CacheSWRSeconds is the SWR override in seconds (0-86400).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=86400
	CacheSWRSeconds *int `json:"cacheSwrSeconds,omitempty"`
}

// CacheRuleAtProvider defines the observed state of a CacheRule.
type CacheRuleAtProvider struct {
	// NormalizedSQL is the normalized SQL text with $N placeholders.
	NormalizedSQL string `json:"normalizedSql,omitempty"`

	// QueryType is the query classification: read, write, or other.
	QueryType string `json:"queryType,omitempty"`

	// CallCount is the total query executions observed.
	CallCount int64 `json:"callCount,omitempty"`

	// AvgLatencyMs is the average query latency in milliseconds.
	AvgLatencyMs float64 `json:"avgLatencyMs,omitempty"`

	// P95LatencyMs is the 95th percentile latency in milliseconds.
	P95LatencyMs float64 `json:"p95LatencyMs,omitempty"`

	// AvgResponseBytes is the average response size in bytes.
	AvgResponseBytes int64 `json:"avgResponseBytes,omitempty"`

	// StabilityRate is the response stability rate (0.0-1.0).
	StabilityRate float64 `json:"stabilityRate,omitempty"`

	// Recommendation is the cache recommendation: great, good, fair, or poor.
	Recommendation string `json:"recommendation,omitempty"`

	// FirstSeenAt is the ISO 8601 timestamp when the query was first observed.
	FirstSeenAt string `json:"firstSeenAt,omitempty"`

	// LastSeenAt is the ISO 8601 timestamp when the query was last observed.
	LastSeenAt string `json:"lastSeenAt,omitempty"`
}

// CacheRuleSpec defines the desired state of a CacheRule.
type CacheRuleSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       CacheRuleForProvider `json:"forProvider"`
}

// CacheRuleStatus defines the observed state of a CacheRule.
type CacheRuleStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          CacheRuleAtProvider `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane;managed;pgbeam
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// CacheRule is a managed resource that represents a PgBeam Cache Rule.
// Deletion is a soft delete that disables caching for the query shape.
type CacheRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CacheRuleSpec   `json:"spec"`
	Status CacheRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CacheRuleList contains a list of CacheRules.
type CacheRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CacheRule `json:"items"`
}

// ============================================================================
// SpendLimit
// ============================================================================

// SpendLimitForProvider defines the desired state of a SpendLimit.
type SpendLimitForProvider struct {
	// OrgID is the organization ID.
	// +kubebuilder:validation:Required
	// +immutable
	OrgID string `json:"orgId"`

	// SpendLimit is the monthly spend limit in dollars. Nil removes the limit.
	// +optional
	SpendLimit *float64 `json:"spendLimit,omitempty"`
}

// PlanLimitsObservation represents the observed plan limits.
type PlanLimitsObservation struct {
	// QueriesPerDay is the daily query limit.
	QueriesPerDay int64 `json:"queriesPerDay,omitempty"`

	// MaxProjects is the max number of projects.
	MaxProjects int `json:"maxProjects,omitempty"`

	// MaxDatabases is the max number of databases.
	MaxDatabases int `json:"maxDatabases,omitempty"`

	// MaxConnections is the max concurrent connections.
	MaxConnections int `json:"maxConnections,omitempty"`

	// QueriesPerSecond is the max QPS.
	QueriesPerSecond int `json:"queriesPerSecond,omitempty"`

	// BytesPerMonth is the monthly transfer limit in bytes.
	BytesPerMonth int64 `json:"bytesPerMonth,omitempty"`

	// MaxQueryShapes is the max distinct query shapes.
	MaxQueryShapes int `json:"maxQueryShapes,omitempty"`

	// IncludedSeats is the number of included team seats.
	IncludedSeats int `json:"includedSeats,omitempty"`
}

// SpendLimitAtProvider defines the observed state of a SpendLimit.
type SpendLimitAtProvider struct {
	// Plan is the current plan tier.
	Plan string `json:"plan,omitempty"`

	// BillingProvider is the billing provider (stripe, vercel, aws).
	BillingProvider string `json:"billingProvider,omitempty"`

	// SubscriptionStatus is the subscription status.
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"`

	// CurrentPeriodEnd is the ISO 8601 end of the current billing period.
	CurrentPeriodEnd string `json:"currentPeriodEnd,omitempty"`

	// Enabled indicates whether billing is active.
	Enabled *bool `json:"enabled,omitempty"`

	// CustomPricing indicates whether custom enterprise pricing is active.
	CustomPricing *bool `json:"customPricing,omitempty"`

	// Limits are the effective usage limits for the plan.
	Limits *PlanLimitsObservation `json:"limits,omitempty"`
}

// SpendLimitSpec defines the desired state of a SpendLimit.
type SpendLimitSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       SpendLimitForProvider `json:"forProvider"`
}

// SpendLimitStatus defines the observed state of a SpendLimit.
type SpendLimitStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SpendLimitAtProvider `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane;managed;pgbeam
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// SpendLimit is a managed resource that represents a PgBeam Spend Limit.
// Deletion is a soft delete that removes the spend limit.
type SpendLimit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpendLimitSpec   `json:"spec"`
	Status SpendLimitStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpendLimitList contains a list of SpendLimits.
type SpendLimitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpendLimit `json:"items"`
}
