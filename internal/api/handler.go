package api

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"zfs-unlocker/internal/approval"
	"zfs-unlocker/internal/config"
	"zfs-unlocker/internal/vault"

	"github.com/gin-gonic/gin"
)

type ClientRule struct {
	AllowedNets []*net.IPNet
	PathPrefix  string
}

type Notifier interface {
	RequestApproval(reqID, description string) error
}

type Handler struct {
	approvalService *approval.Service
	vaultClient     vault.Client
	bot             Notifier
	clientRules     map[string]*ClientRule
}

func New(apiKeys []config.APIKey, approvalSvc *approval.Service, vaultClient vault.Client, bot Notifier) *Handler {
	rules := make(map[string]*ClientRule)

	for _, k := range apiKeys {
		rule := &ClientRule{
			PathPrefix: k.PathPrefix,
		}
		if len(k.AllowedCIDRs) > 0 {
			for _, cidr := range k.AllowedCIDRs {
				_, network, err := net.ParseCIDR(cidr)
				if err != nil {
					log.Printf("Warning: Invalid CIDR %s for API key %s: %v", cidr, k.Key, err)
					continue
				}
				rule.AllowedNets = append(rule.AllowedNets, network)
			}
		}
		rules[k.Key] = rule
	}

	return &Handler{
		approvalService: approvalSvc,
		vaultClient:     vaultClient,
		bot:             bot,
		clientRules:     rules,
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Route: /unlock/:apiKey/:volumeID
	r.GET("/unlock/:apiKey/:volumeID", h.authMiddleware, h.handleUnlock)
	r.POST("/unlock/:apiKey/:volumeID", h.authMiddleware, h.handleUnlock)
}

func (h *Handler) authMiddleware(c *gin.Context) {
	apiKey := c.Param("apiKey")
	if apiKey == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing key parameter"})
		return
	}

	rule, exists := h.clientRules[apiKey]
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Check IP restrictions
	if len(rule.AllowedNets) > 0 {
		clientIPStr := c.ClientIP()
		clientIP := net.ParseIP(clientIPStr)

		if clientIP == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid IP"})
			return
		}

		allowed := false
		for _, network := range rule.AllowedNets {
			if network.Contains(clientIP) {
				allowed = true
				break
			}
		}

		if !allowed {
			log.Printf("Access denied for key %s from IP %s", apiKey, clientIPStr)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "IP not allowed"})
			return
		}
	}

	// Store rule info in context for the handler
	c.Set("clientRule", rule)
	c.Next()
}

func (h *Handler) handleUnlock(c *gin.Context) {
	ruleObj, _ := c.Get("clientRule")
	rule := ruleObj.(*ClientRule)

	volumeID := c.Param("volumeID")
	if volumeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing volume ID"})
		return
	}

	// 1. Create request
	reqID, waitChan := h.approvalService.NewRequest()

	// 2. Notify via Telegram
	msg := fmt.Sprintf("Request to unlock volume: `%s`", volumeID)
	err := h.bot.RequestApproval(reqID, msg)
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
			// Uses stored PathPrefix from config and extracted VolumeID
			secret, err := h.vaultClient.GetSecret(c.Request.Context(), rule.PathPrefix, volumeID)
			if err != nil {
				log.Printf("Vault fetch failed: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Approved, but failed to fetch secret"})
				return
			}

			// ZFS Compatibility: Return raw text found in standard fields
			if val, ok := secret["key"]; ok {
				c.String(http.StatusOK, fmt.Sprintf("%v", val))
				return
			}

			// Fallback: If we can't find a single key, return JSON (useful for debugging)
			c.JSON(http.StatusOK, gin.H{"status": "approved", "secret": secret})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"status": "denied"})
		}
	case <-time.After(5 * time.Minute): // Timeout
		h.approvalService.ResolveRequest(reqID, false)
		c.JSON(http.StatusGatewayTimeout, gin.H{"status": "timeout"})
	}
}
