package controller

import (
	"context"
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/sferarc/pgbeam-crossplane/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errGetProviderConfig  = "cannot get ProviderConfig"
	errGetAPIKeySecret    = "cannot get API key secret"
	errTrackProviderUsage = "cannot track ProviderConfig usage"
)

// connector produces external clients by reading the ProviderConfig.
type connector struct {
	kube  client.Client
	usage resource.Tracker
}

// getClient reads the ProviderConfig and creates a pgbeam-go client.
func (c *connector) getClient(ctx context.Context, mg resource.Managed) (*pgbeam.Client, error) {
	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, fmt.Errorf("%s: %w", errTrackProviderUsage, err)
	}

	pc := &v1alpha1.ProviderConfig{}
	pcRef := mg.GetProviderConfigReference()
	if pcRef == nil {
		return nil, fmt.Errorf("%s: no providerConfigRef set", errGetProviderConfig)
	}

	if err := c.kube.Get(ctx, types.NamespacedName{Name: pcRef.Name}, pc); err != nil {
		return nil, fmt.Errorf("%s: %w", errGetProviderConfig, err)
	}

	apiKey, err := c.getAPIKey(ctx, pc.Spec.APIKeySecretRef)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetAPIKeySecret, err)
	}

	return pgbeam.NewClient(&pgbeam.ClientOptions{
		APIKey:  apiKey,
		BaseURL: pc.Spec.BaseURL,
	}), nil
}

// getAPIKey reads the API key and strips surrounding whitespace, since secrets
// created from a file or with a plain echo carry a trailing newline that
// net/http rejects in the Authorization header.
func (c *connector) getAPIKey(ctx context.Context, ref xpv1.SecretKeySelector) (string, error) {
	val, err := c.getSecretValue(ctx, ref)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(val)
	if key == "" {
		return "", fmt.Errorf("key %q in secret %s/%s is empty", ref.Key, ref.Namespace, ref.Name)
	}
	return key, nil
}

// getSecretValue reads a secret key selector and returns the value as a string.
func (c *connector) getSecretValue(ctx context.Context, ref xpv1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	nn := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	if err := c.kube.Get(ctx, nn, secret); err != nil {
		return "", fmt.Errorf("get secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	val, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %s/%s", ref.Key, ref.Namespace, ref.Name)
	}
	return string(val), nil
}
