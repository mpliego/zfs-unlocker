package telegram

import (
	"fmt"
	"log"
	"os"
	"strings"

	"zfs-unlocker/internal/approval"
	"zfs-unlocker/internal/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api             *tgbotapi.BotAPI
	approvalService *approval.Service
	chatID          int64
}

func New(cfg config.TelegramConfig, approvalService *approval.Service) (*Bot, error) {
	token := cfg.BotToken
	if token == "" {
		token = os.Getenv("TELEGRAM_BOT_TOKEN")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	// bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	return &Bot{
		api:             bot,
		approvalService: approvalService,
		chatID:          cfg.ChatID,
	}, nil
}

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			if update.CallbackQuery != nil {
				b.handleCallback(update.CallbackQuery)
				continue
			}
			// Handle other messages if needed
		}
	}()
}

// RequestApproval sends a message with inline buttons to approve/deny
func (b *Bot) RequestApproval(reqID string, description string) error {
	msg := tgbotapi.NewMessage(b.chatID, fmt.Sprintf("🔓 *Unlock Request*\nID: `%s`\nInfo: %s", reqID, description))
	msg.ParseMode = "Markdown"

	approveBtn := tgbotapi.NewInlineKeyboardButtonData("✅ Approve", fmt.Sprintf("approve:%s", reqID))
	denyBtn := tgbotapi.NewInlineKeyboardButtonData("❌ Deny", fmt.Sprintf("deny:%s", reqID))

	row := tgbotapi.NewInlineKeyboardRow(approveBtn, denyBtn)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row)

	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		return
	}

	action := parts[0]
	reqID := parts[1]

	var responseText string
	var success bool

	switch action {
	case "approve":
		success = b.approvalService.ResolveRequest(reqID, true)
		if success {
			responseText = fmt.Sprintf("✅ Request %s Approved", reqID)
		} else {
			responseText = "⚠️ Request expired or not found"
		}
	case "deny":
		success = b.approvalService.ResolveRequest(reqID, false)
		if success {
			responseText = fmt.Sprintf("❌ Request %s Denied", reqID)
		} else {
			responseText = "⚠️ Request expired or not found"
		}
	}

	// Answer callback to stop loading animation
	callbackCfg := tgbotapi.NewCallback(cb.ID, responseText)
	if _, err := b.api.Request(callbackCfg); err != nil {
		log.Printf("Failed to answer callback: %v", err)
	}

	// Update the message to remove buttons and show status
	if success {
		edit := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, responseText)
		if _, err := b.api.Send(edit); err != nil {
			log.Printf("Failed to edit message: %v", err)
		}
	}
}
