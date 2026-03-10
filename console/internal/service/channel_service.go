package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/smallnest/goclaw/console/internal/model"
	goclawConfig "github.com/smallnest/goclaw/config"
)

// ChannelService manages channel configurations using goclaw config
type ChannelService struct {
	config     *goclawConfig.Config
	configPath string
	mu         sync.RWMutex
}

// NewChannelService creates a new channel service
func NewChannelService(cfg *goclawConfig.Config, workspace string) *ChannelService {
	configPath := filepath.Join(workspace, "config.json")
	return &ChannelService{
		config:     cfg,
		configPath: configPath,
	}
}

// GetChannelTypes returns available channel types (matches Python BUILTIN_CHANNEL_TYPES)
func (s *ChannelService) GetChannelTypes() model.ChannelTypesResponse {
	return model.ChannelTypesResponse{
		"imessage",
		"discord",
		"dingtalk",
		"feishu",
		"qq",
		"telegram",
		"console",
		"whatsapp",
		"wework",
		"infoflow",
	}
}

// GetChannels returns all channel configurations (matches Python GET /config/channels)
func (s *ChannelService) GetChannels() *model.ChannelsConfigResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := &model.ChannelsConfigResponse{}

	if s.config == nil {
		return result
	}

	// Telegram
	if s.config.Channels.Telegram.Enabled || s.config.Channels.Telegram.Token != "" {
		result.Telegram = &model.TelegramChannelConfig{
			Enabled:    s.config.Channels.Telegram.Enabled,
			BotToken:   s.config.Channels.Telegram.Token,
			AllowedIDs: s.config.Channels.Telegram.AllowedIDs,
		}
	}

	// WhatsApp
	if s.config.Channels.WhatsApp.Enabled || s.config.Channels.WhatsApp.BridgeURL != "" {
		result.WhatsApp = &model.WhatsAppChannelConfig{
			Enabled:   s.config.Channels.WhatsApp.Enabled,
			BridgeURL: s.config.Channels.WhatsApp.BridgeURL,
			AllowedIDs: s.config.Channels.WhatsApp.AllowedIDs,
		}
	}

	// Feishu
	if s.config.Channels.Feishu.Enabled || s.config.Channels.Feishu.AppID != "" {
		result.Feishu = &model.FeishuChannelConfig{
			Enabled:           s.config.Channels.Feishu.Enabled,
			AppID:             s.config.Channels.Feishu.AppID,
			AppSecret:         s.config.Channels.Feishu.AppSecret,
			EncryptKey:        s.config.Channels.Feishu.EncryptKey,
			VerificationToken: s.config.Channels.Feishu.VerificationToken,
			Domain:            s.config.Channels.Feishu.Domain,
			AllowedIDs:        s.config.Channels.Feishu.AllowedIDs,
		}
	}

	// DingTalk
	if s.config.Channels.DingTalk.Enabled || s.config.Channels.DingTalk.ClientID != "" {
		result.DingTalk = &model.DingTalkChannelConfig{
			Enabled:      s.config.Channels.DingTalk.Enabled,
			ClientID:     s.config.Channels.DingTalk.ClientID,
			ClientSecret: s.config.Channels.DingTalk.ClientSecret,
			AllowedIDs:   s.config.Channels.DingTalk.AllowedIDs,
		}
	}

	// QQ
	if s.config.Channels.QQ.Enabled || s.config.Channels.QQ.AppID != "" {
		result.QQ = &model.QQChannelConfig{
			Enabled:    s.config.Channels.QQ.Enabled,
			AppID:      s.config.Channels.QQ.AppID,
			AppSecret:  s.config.Channels.QQ.AppSecret,
			AllowedIDs: s.config.Channels.QQ.AllowedIDs,
		}
	}

	// WeWork
	if s.config.Channels.WeWork.Enabled || s.config.Channels.WeWork.CorpID != "" {
		result.WeWork = &model.WeWorkChannelConfig{
			Enabled:        s.config.Channels.WeWork.Enabled,
			CorpID:         s.config.Channels.WeWork.CorpID,
			AgentID:        s.config.Channels.WeWork.AgentID,
			Secret:         s.config.Channels.WeWork.Secret,
			Token:          s.config.Channels.WeWork.Token,
			EncodingAESKey: s.config.Channels.WeWork.EncodingAESKey,
			AllowedIDs:     s.config.Channels.WeWork.AllowedIDs,
		}
	}

	// Infoflow
	if s.config.Channels.Infoflow.Enabled || s.config.Channels.Infoflow.WebhookURL != "" {
		result.Infoflow = &model.InfoflowChannelConfig{
			Enabled:     s.config.Channels.Infoflow.Enabled,
			WebhookURL:  s.config.Channels.Infoflow.WebhookURL,
			Token:       s.config.Channels.Infoflow.Token,
			AESKey:      s.config.Channels.Infoflow.AESKey,
			WebhookPort: s.config.Channels.Infoflow.WebhookPort,
			AllowedIDs:  s.config.Channels.Infoflow.AllowedIDs,
		}
	}

	// Console is always available
	result.Console = &model.ConsoleChannelConfig{
		Enabled:   true,
		BotPrefix: "[BOT] ",
	}

	return result
}

// UpdateChannels updates all channel configurations (matches Python PUT /config/channels)
func (s *ChannelService) UpdateChannels(config *model.ChannelsConfigUpdateRequest) *model.ChannelsConfigResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return &model.ChannelsConfigResponse{}
	}

	// Update each channel from the request
	for channelName, channelConfig := range *config {
		configMap, ok := channelConfig.(map[string]interface{})
		if !ok {
			continue
		}

		switch channelName {
		case "telegram":
			s.updateTelegramConfig(configMap)
		case "whatsapp":
			s.updateWhatsAppConfig(configMap)
		case "feishu":
			s.updateFeishuConfig(configMap)
		case "dingtalk":
			s.updateDingTalkConfig(configMap)
		case "qq":
			s.updateQQConfig(configMap)
		case "wework":
			s.updateWeWorkConfig(configMap)
		case "infoflow":
			s.updateInfoflowConfig(configMap)
		}
	}

	// Save config
	s.saveConfig()

	return s.GetChannels()
}

// GetChannel returns a specific channel configuration (matches Python GET /config/channels/{channel_name})
func (s *ChannelService) GetChannel(name string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channels := s.GetChannels()

	switch name {
	case "telegram":
		return channels.Telegram, nil
	case "whatsapp":
		return channels.WhatsApp, nil
	case "feishu":
		return channels.Feishu, nil
	case "dingtalk":
		return channels.DingTalk, nil
	case "qq":
		return channels.QQ, nil
	case "wework":
		return channels.WeWork, nil
	case "infoflow":
		return channels.Infoflow, nil
	case "console":
		return channels.Console, nil
	default:
		return nil, ErrChannelNotFound
	}
}

// UpdateChannel updates a specific channel configuration (matches Python PUT /config/channels/{channel_name})
func (s *ChannelService) UpdateChannel(name string, config map[string]interface{}) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return nil, ErrChannelNotFound
	}

	switch name {
	case "telegram":
		s.updateTelegramConfig(config)
	case "whatsapp":
		s.updateWhatsAppConfig(config)
	case "feishu":
		s.updateFeishuConfig(config)
	case "dingtalk":
		s.updateDingTalkConfig(config)
	case "qq":
		s.updateQQConfig(config)
	case "wework":
		s.updateWeWorkConfig(config)
	case "infoflow":
		s.updateInfoflowConfig(config)
	default:
		return nil, ErrChannelNotFound
	}

	// Save config
	s.saveConfig()

	return s.GetChannel(name)
}

// Helper functions to update individual channel configs

func (s *ChannelService) updateTelegramConfig(config map[string]interface{}) {
	if v, ok := config["enabled"].(bool); ok {
		s.config.Channels.Telegram.Enabled = v
	}
	if v, ok := config["bot_token"].(string); ok {
		s.config.Channels.Telegram.Token = v
	}
	if _, ok := config["bot_prefix"].(string); ok {
		// bot_prefix is not in goclaw config, but we can store it in metadata
	}
	if _, ok := config["http_proxy"].(string); ok {
		// http_proxy is not in goclaw config
	}
	if _, ok := config["show_typing"].(bool); ok {
		// show_typing is not in goclaw config
	}
	if v, ok := config["allowed_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(v))
		for _, id := range v {
			if str, ok := id.(string); ok {
				ids = append(ids, str)
			}
		}
		s.config.Channels.Telegram.AllowedIDs = ids
	}
}

func (s *ChannelService) updateWhatsAppConfig(config map[string]interface{}) {
	if v, ok := config["enabled"].(bool); ok {
		s.config.Channels.WhatsApp.Enabled = v
	}
	if v, ok := config["bridge_url"].(string); ok {
		s.config.Channels.WhatsApp.BridgeURL = v
	}
	if v, ok := config["allowed_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(v))
		for _, id := range v {
			if str, ok := id.(string); ok {
				ids = append(ids, str)
			}
		}
		s.config.Channels.WhatsApp.AllowedIDs = ids
	}
}

func (s *ChannelService) updateFeishuConfig(config map[string]interface{}) {
	if v, ok := config["enabled"].(bool); ok {
		s.config.Channels.Feishu.Enabled = v
	}
	if v, ok := config["app_id"].(string); ok {
		s.config.Channels.Feishu.AppID = v
	}
	if v, ok := config["app_secret"].(string); ok {
		s.config.Channels.Feishu.AppSecret = v
	}
	if v, ok := config["encrypt_key"].(string); ok {
		s.config.Channels.Feishu.EncryptKey = v
	}
	if v, ok := config["verification_token"].(string); ok {
		s.config.Channels.Feishu.VerificationToken = v
	}
	if v, ok := config["domain"].(string); ok {
		s.config.Channels.Feishu.Domain = v
	}
	if v, ok := config["allowed_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(v))
		for _, id := range v {
			if str, ok := id.(string); ok {
				ids = append(ids, str)
			}
		}
		s.config.Channels.Feishu.AllowedIDs = ids
	}
}

func (s *ChannelService) updateDingTalkConfig(config map[string]interface{}) {
	if v, ok := config["enabled"].(bool); ok {
		s.config.Channels.DingTalk.Enabled = v
	}
	if v, ok := config["client_id"].(string); ok {
		s.config.Channels.DingTalk.ClientID = v
	}
	if v, ok := config["client_secret"].(string); ok {
		s.config.Channels.DingTalk.ClientSecret = v
	}
	if v, ok := config["allowed_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(v))
		for _, id := range v {
			if str, ok := id.(string); ok {
				ids = append(ids, str)
			}
		}
		s.config.Channels.DingTalk.AllowedIDs = ids
	}
}

func (s *ChannelService) updateQQConfig(config map[string]interface{}) {
	if v, ok := config["enabled"].(bool); ok {
		s.config.Channels.QQ.Enabled = v
	}
	if v, ok := config["app_id"].(string); ok {
		s.config.Channels.QQ.AppID = v
	}
	if v, ok := config["app_secret"].(string); ok {
		s.config.Channels.QQ.AppSecret = v
	}
	if v, ok := config["allowed_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(v))
		for _, id := range v {
			if str, ok := id.(string); ok {
				ids = append(ids, str)
			}
		}
		s.config.Channels.QQ.AllowedIDs = ids
	}
}

func (s *ChannelService) updateWeWorkConfig(config map[string]interface{}) {
	if v, ok := config["enabled"].(bool); ok {
		s.config.Channels.WeWork.Enabled = v
	}
	if v, ok := config["corp_id"].(string); ok {
		s.config.Channels.WeWork.CorpID = v
	}
	if v, ok := config["agent_id"].(string); ok {
		s.config.Channels.WeWork.AgentID = v
	}
	if v, ok := config["secret"].(string); ok {
		s.config.Channels.WeWork.Secret = v
	}
	if v, ok := config["token"].(string); ok {
		s.config.Channels.WeWork.Token = v
	}
	if v, ok := config["encoding_aes_key"].(string); ok {
		s.config.Channels.WeWork.EncodingAESKey = v
	}
	if v, ok := config["allowed_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(v))
		for _, id := range v {
			if str, ok := id.(string); ok {
				ids = append(ids, str)
			}
		}
		s.config.Channels.WeWork.AllowedIDs = ids
	}
}

func (s *ChannelService) updateInfoflowConfig(config map[string]interface{}) {
	if v, ok := config["enabled"].(bool); ok {
		s.config.Channels.Infoflow.Enabled = v
	}
	if v, ok := config["webhook_url"].(string); ok {
		s.config.Channels.Infoflow.WebhookURL = v
	}
	if v, ok := config["token"].(string); ok {
		s.config.Channels.Infoflow.Token = v
	}
	if v, ok := config["aes_key"].(string); ok {
		s.config.Channels.Infoflow.AESKey = v
	}
	if v, ok := config["webhook_port"].(float64); ok {
		s.config.Channels.Infoflow.WebhookPort = int(v)
	}
	if v, ok := config["allowed_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(v))
		for _, id := range v {
			if str, ok := id.(string); ok {
				ids = append(ids, str)
			}
		}
		s.config.Channels.Infoflow.AllowedIDs = ids
	}
}

// saveConfig saves the configuration to file
func (s *ChannelService) saveConfig() error {
	if s.configPath == "" {
		return nil
	}

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configPath, data, 0644)
}

// ErrChannelNotFound is returned when a channel is not found
var ErrChannelNotFound = &ChannelNotFoundError{}

type ChannelNotFoundError struct{}

func (e *ChannelNotFoundError) Error() string {
	return "channel not found"
}
