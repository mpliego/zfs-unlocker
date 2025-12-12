package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"zfs-unlocker/internal/api"
	"zfs-unlocker/internal/approval"
	"zfs-unlocker/internal/config"
	"zfs-unlocker/internal/telegram"
	"zfs-unlocker/internal/vault"

	"github.com/gin-gonic/gin"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Parse flags manually or using flag package.
	versionFlag := flag.Bool("version", false, "Print version information")
	vFlag := flag.Bool("v", false, "Print version information")
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	if *versionFlag || *vFlag || (len(flag.Args()) > 0 && flag.Args()[0] == "version") {
		fmt.Printf("zfs-unlocker %s\n", version)
		os.Exit(0)
	}
	// 1. Load Config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Vault
	vaultSvc, err := vault.New(cfg.Vault)
	if err != nil {
		log.Fatalf("Failed to initialize Vault service: %v", err)
	}

	// 3. Initialize Approval Service
	approvalSvc := approval.New()

	// 4. Initialize Telegram Bot
	botSvc, err := telegram.New(cfg.Telegram, approvalSvc)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}
	botSvc.Start()

	// 5. Initialize API
	apiHandler := api.New(cfg.ApiKeys, approvalSvc, vaultSvc, botSvc)

	// 6. Setup Router
	r := gin.Default()
	apiHandler.RegisterRoutes(r)

	// 7. Run Server
	addr := cfg.Server.ListenAddress
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
