package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"zfs-unlocker/internal/approval"
	"zfs-unlocker/internal/telegram"
	"zfs-unlocker/internal/vault"
)

type Handler struct {
	approvalService *approval.Service
	vaultClient     vault.Client // Use interface
	bot             *telegram.Bot
	apiKeys         map[string]bool
}

func New(validKeys []string, approvalSvc *approval.Service, vaultClient vault.Client, bot *telegram.Bot) *Handler {
	keys := make(map[string]bool)
	for _, k := range validKeys {
		keys[k] = true
	}

	return &Handler{
		approvalService: approvalSvc,
		vaultClient:     vaultClient,
		bot:             bot,
		apiKeys:         keys,
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/unlock", h.authMiddleware, h.handleUnlock)
}

func (h *Handler) authMiddleware(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	if !h.apiKeys[apiKey] {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	c.Next()
}

func (h *Handler) handleUnlock(c *gin.Context) {
	// 1. Create request
	reqID, waitChan := h.approvalService.NewRequest()

	// 2. Notify via Telegram
	err := h.bot.RequestApproval(reqID, "Request to unlock ZFS dataset")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send approval request"})
		h.approvalService.ResolveRequest(reqID, false) // cleanup
		return
	}

	// 3. Wait for decision
	select {
	case approved := <-waitChan:
		if approved {
			// Retrieve secret from Vault
			secret, err := h.vaultClient.GetSecret(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Approved, but failed to fetch secret", "details": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "approved", "secret": secret})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"status": "denied"})
		}
	case <-time.After(5 * time.Minute): // Timeout
		h.approvalService.ResolveRequest(reqID, false)
		c.JSON(http.StatusGatewayTimeout, gin.H{"status": "timeout"})
	}
}
