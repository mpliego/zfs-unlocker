package vault

import (
	"context"
	"fmt"

	"zfs-unlocker/internal/config"

	hashivault "github.com/hashicorp/vault/api"
)

type Client interface {
	GetSecret(ctx context.Context) (map[string]interface{}, error)
}

type VaultClient struct {
	client     *hashivault.Client
	mountPath  string
	secretPath string
}

func New(cfg config.VaultConfig) (*VaultClient, error) {
	vConfig := hashivault.DefaultConfig()
	vConfig.Address = cfg.Address

	client, err := hashivault.NewClient(vConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	client.SetToken(cfg.Token)

	// Verify connection/token (optional, but good practice)
	// _, err = client.Auth().Token().LookupSelf()
	// if err != nil {
	// 	 return nil, fmt.Errorf("token validation failed: %w", err)
	// }

	return &VaultClient{
		client:     client,
		mountPath:  cfg.MountPath,
		secretPath: cfg.SecretPath,
	}, nil
}

func (v *VaultClient) GetSecret(ctx context.Context) (map[string]interface{}, error) {
	// Access KV v2
	// The path for KV v2 reads usually involves "data" - SDK handles some,
	// but using logical read on mountPath/data/secretPath is standard for KV2 key reading.
	// NOTE: standard client.KVv2 call is preferred for abstraction.

	secret, err := v.client.KVv2(v.mountPath).Get(ctx, v.secretPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read secret: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found at %s/%s", v.mountPath, v.secretPath)
	}

	return secret.Data, nil
}
