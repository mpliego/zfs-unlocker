# ZFS Unlocker

ZFS Unlocker is a secure, modular microservice designed to facilitate the remote unlocking of ZFS encrypted datasets. It acts as a middleman between your ZFS servers and a HashiCorp Vault, requiring manual approval via Telegram for every key retrieval request.

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Vault](https://img.shields.io/badge/hashicorp%20vault-%23C4C7E5.svg?style=for-the-badge&logo=hashicorpvault&logoColor=black)
![Telegram](https://img.shields.io/badge/Telegram-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)

## Features

*   **Human-in-the-loop Security**: Every key request triggers a Telegram message with "Approve" and "Deny" buttons. The request hangs until approved.
*   **HashiCorp Vault Integration**: Fetches encryption keys securely from a Vault KV-v2 engine.
*   **IP Allowlisting**: Restrict API keys to specific CIDR ranges (e.g., your ZFS server's internal IP).
*   **Dynamic Paths**: Maps API keys to specific Vault sub-paths for multi-tenant or multi-server support.
*   **ZFS Compatibility**: Designed to work as a `keysource` for `zfs load-key` fetching from a URL.

## Workflow

1.  **Request**: Client (ZFS server) makes a minimal GET request:
    `GET /unlock/{api_key}/{volume_id}`
2.  **Auth & Routing**: Server validates the API Key and Client IP.
3.  **Notification**: A Telegram message is sent to the configured Chat ID:
    > "Request to unlock volume: data/pool/secure"
4.  **Approval**: Admin clicks "✅ Approve".
5.  **Fetch**: Server fetches the secret from Vault at:
    `{vault_path}/{key_prefix}/{volume_id}`
6.  **Response**: The key is returned to the client as JSON.

## Configuration

Create a `config.yaml` file in the working directory:

```yaml
server:
  listen_address: ":8080"    # Optional: Defaults to :8080
  # cert_file: "server.crt"  # Optional: Enable TLS
  # key_file: "server.key"   # Optional: Enable TLS

vault:
  address: "http://127.0.0.1:8200"
  mount_path: "secret"       # The KV-v2 mount point
  secret_path: "zfs-keys"    # The base path for keys
  # token: "..."             # Optional: Can be set via VAULT_TOKEN env var

telegram:
  chat_id: 123456789
  # bot_token: "..."         # Optional: Can be set via TELEGRAM_BOT_TOKEN env var

api_keys:
  - key: "server-01-api-key"
    path_prefix: "server-01" # Sub-path in Vault
    allowed_cidrs:
      - "192.168.1.10/32"    # Only allow requests from this IP
  - key: "nas-backup-key"
    path_prefix: "backup-node"
    allowed_cidrs:
      - "10.0.0.0/8"
```

### Environment Variables
For better security, you can provide secrets via environment variables instead of the config file:
*   `VAULT_TOKEN`: Authentication token for HashiCorp Vault.
*   `TELEGRAM_BOT_TOKEN`: The API token for your Telegram Bot.

## Usage

### 1. Run the Server
```bash
# Build
go build -o zfs-unlocker ./cmd/server

# Run with default config (config.yaml)
./zfs-unlocker

# Run with custom config
./zfs-unlocker --config /etc/zfs-unlocker/production.yaml

# Check Version
./zfs-unlocker version
```

### 2. Client Request (Example)
Using `curl` to simulate a ZFS key load:

```bash
curl -s "http://localhost:8080/unlock/server-01-api-key/tank-secure-dataset"
```

Based on the config above, this will attempt to fetch the secret from Vault at:
`secret/data/zfs-keys/server-01/tank-secure-dataset`

## API Reference

### `GET /unlock/:apiKey/:volumeID`

*   **apiKey**: The authentication token configured in `config.yaml`.
*   **volumeID**: The identifier for the volume (used to find the key in Vault).

**Response (Success 200)**

*   **Raw Binary**: The server assumes that the secret stored in Vault is a **Base64 encoded string**. It automatically decodes this value and returns the raw binary bytes. This makes it compatible with `keyformat=raw`.
*   **JSON**: If no standard key field is found, it falls back to returning the full secret JSON object.

```bash
# Fetch raw key (server decodes Base64 from Vault automatically)
zfs load-key -L "https://zfs-unlocker/unlock/key/vol" pool/dataset
```

**Response (Pending)**
The connection will remain open (blocking) until the admin clicks a button in Telegram or the timeout (5 minutes) is reached.

## Development

**Run tests:**
```bash
go test -v ./...
```

## License

MIT
