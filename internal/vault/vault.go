package vault

import (
	"context"
	"fmt"

	"zfs-unlocker/internal/config"

	hashivault "github.com/hashicorp/vault/api"
)

type Client interface {
	GetSecret(ctx context.Context, keyPrefix, volumeID string) (map[string]interface{}, error)
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

	// Prioritize config file token, fallback to standard VAULT_TOKEN env var
	if cfg.Token != "" {
		client.SetToken(cfg.Token)
	}

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

func (v *VaultClient) GetSecret(ctx context.Context, keyPrefix, volumeID string) (map[string]interface{}, error) {
	// Path construction: {vault-config-prefix}/{api-key-config-prefix}/{volume-id}
	// e.g. secret/data/my-secret/key-prefix/volume-id
	// Note: KVv2 Get argument is relative to the mount.
	// If mount is "secret", and we want "secret/data/foo/bar", we ask for "foo/bar".

	// Assuming v.secretPath is the "vault-config-prefix" (base path)
	fullPath := fmt.Sprintf("%s/%s/%s", v.secretPath, keyPrefix, volumeID)
	// Clean up double slashes if any prefix is empty
	// (Simple string manip or path.Join, but path.Join might mess with URL schemes if any, keeping simple for now)

	secret, err := v.client.KVv2(v.mountPath).Get(ctx, fullPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read secret at %s: %w", fullPath, err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found at %s/%s", v.mountPath, fullPath)
	}

	return secret.Data, nil
}
