package model

// ChannelTypesResponse represents available channel types
type ChannelTypesResponse []string

// ChannelsConfigResponse represents all channels configuration (matches Python response)
type ChannelsConfigResponse struct {
	IMessage  *IMessageChannelConfig  `json:"imessage,omitempty"`
	Discord   *DiscordChannelConfig   `json:"discord,omitempty"`
	DingTalk  *DingTalkChannelConfig  `json:"dingtalk,omitempty"`
	Feishu    *FeishuChannelConfig    `json:"feishu,omitempty"`
	QQ        *QQChannelConfig        `json:"qq,omitempty"`
	Telegram  *TelegramChannelConfig  `json:"telegram,omitempty"`
	Console   *ConsoleChannelConfig   `json:"console,omitempty"`
	WhatsApp  *WhatsAppChannelConfig  `json:"whatsapp,omitempty"`
	WeWork    *WeWorkChannelConfig    `json:"wework,omitempty"`
	Infoflow  *InfoflowChannelConfig  `json:"infoflow,omitempty"`
}

// IMessageChannelConfig iMessage channel configuration
type IMessageChannelConfig struct {
	Enabled   bool    `json:"enabled"`
	BotPrefix string  `json:"bot_prefix"`
	DBPath    string  `json:"db_path"`
	PollSec   float64 `json:"poll_sec"`
}

// DiscordChannelConfig Discord channel configuration
type DiscordChannelConfig struct {
	Enabled       bool   `json:"enabled"`
	BotPrefix     string `json:"bot_prefix"`
	BotToken      string `json:"bot_token"`
	HTTPProxy     string `json:"http_proxy"`
	HTTPProxyAuth string `json:"http_proxy_auth"`
}

// DingTalkChannelConfig DingTalk channel configuration
type DingTalkChannelConfig struct {
	Enabled      bool     `json:"enabled"`
	BotPrefix    string   `json:"bot_prefix"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	MediaDir     string   `json:"media_dir"`
	AllowedIDs   []string `json:"allowed_ids,omitempty"`
}

// FeishuChannelConfig Feishu channel configuration
type FeishuChannelConfig struct {
	Enabled           bool     `json:"enabled"`
	BotPrefix         string   `json:"bot_prefix"`
	AppID             string   `json:"app_id"`
	AppSecret         string   `json:"app_secret"`
	EncryptKey        string   `json:"encrypt_key"`
	VerificationToken string   `json:"verification_token"`
	MediaDir          string   `json:"media_dir"`
	Domain            string   `json:"domain"`
	AllowedIDs        []string `json:"allowed_ids,omitempty"`
}

// QQChannelConfig QQ channel configuration
type QQChannelConfig struct {
	Enabled    bool     `json:"enabled"`
	BotPrefix  string   `json:"bot_prefix"`
	AppID      string   `json:"app_id"`
	AppSecret  string   `json:"app_secret"`
	AllowedIDs []string `json:"allowed_ids,omitempty"`
}

// TelegramChannelConfig Telegram channel configuration
type TelegramChannelConfig struct {
	Enabled       bool     `json:"enabled"`
	BotPrefix     string   `json:"bot_prefix"`
	BotToken      string   `json:"bot_token"`
	HTTPProxy     string   `json:"http_proxy"`
	HTTPProxyAuth string   `json:"http_proxy_auth"`
	ShowTyping    bool     `json:"show_typing"`
	AllowedIDs    []string `json:"allowed_ids,omitempty"`
}

// ConsoleChannelConfig Console channel configuration
type ConsoleChannelConfig struct {
	Enabled   bool   `json:"enabled"`
	BotPrefix string `json:"bot_prefix"`
}

// WhatsAppChannelConfig WhatsApp channel configuration
type WhatsAppChannelConfig struct {
	Enabled   bool     `json:"enabled"`
	BotPrefix string   `json:"bot_prefix"`
	BridgeURL string   `json:"bridge_url"`
	AllowedIDs []string `json:"allowed_ids,omitempty"`
}

// WeWorkChannelConfig WeWork (企业微信) channel configuration
type WeWorkChannelConfig struct {
	Enabled        bool     `json:"enabled"`
	BotPrefix      string   `json:"bot_prefix"`
	CorpID         string   `json:"corp_id"`
	AgentID        string   `json:"agent_id"`
	Secret         string   `json:"secret"`
	Token          string   `json:"token"`
	EncodingAESKey string   `json:"encoding_aes_key"`
	AllowedIDs     []string `json:"allowed_ids,omitempty"`
}

// InfoflowChannelConfig Infoflow channel configuration
type InfoflowChannelConfig struct {
	Enabled     bool     `json:"enabled"`
	BotPrefix   string   `json:"bot_prefix"`
	WebhookURL  string   `json:"webhook_url"`
	Token       string   `json:"token"`
	AESKey      string   `json:"aes_key"`
	WebhookPort int      `json:"webhook_port"`
	AllowedIDs  []string `json:"allowed_ids,omitempty"`
}

// ChannelConfigUpdateRequest represents a request to update a single channel
// Uses dynamic fields based on channel type
type ChannelConfigUpdateRequest map[string]interface{}

// ChannelsConfigUpdateRequest represents a request to update all channels
type ChannelsConfigUpdateRequest map[string]interface{}
