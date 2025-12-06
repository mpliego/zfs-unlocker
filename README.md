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
vault:
  address: "http://127.0.0.1:8200"
  token: "hvs.your-vault-token"
  mount_path: "secret"       # The KV-v2 mount point
  secret_path: "zfs-keys"    # The base path for keys

telegram:
  bot_token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
  chat_id: 123456789

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

## Usage

### 1. Run the Server
```bash
# Build
go build -o zfs-unlocker ./cmd/server

# Run
./zfs-unlocker
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
```json
{
  "status": "approved",
  "secret": {
    "value": "super-secret-key-material"
  }
}
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
