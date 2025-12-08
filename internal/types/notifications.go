package types

// NotificationsConfig holds all notification channel configurations
type NotificationsConfig struct {
	Slack   NotifySlackConfig   `yaml:"slack"`
	Discord NotifyDiscordConfig `yaml:"discord"`
	Email   NotifyEmailConfig   `yaml:"email"`
}

// NotifySlackConfig holds Slack webhook settings
type NotifySlackConfig struct {
	Enabled     bool     `yaml:"enabled"`
	WebhookURL  string   `yaml:"webhook_url"`
	Channel     string   `yaml:"channel"`
	Username    string   `yaml:"username"`
	IconEmoji   string   `yaml:"icon_emoji"`
	EventTypes  []string `yaml:"events"`       // message, agent_signal, alert, task, recon
	MinPriority int      `yaml:"min_priority"` // 1=critical only, 2=high+, 3=normal+
}

// NotifyDiscordConfig holds Discord webhook settings
type NotifyDiscordConfig struct {
	Enabled     bool     `yaml:"enabled"`
	WebhookURL  string   `yaml:"webhook_url"`
	Username    string   `yaml:"username"`
	AvatarURL   string   `yaml:"avatar_url"`
	EventTypes  []string `yaml:"events"`
	MinPriority int      `yaml:"min_priority"`
}

// NotifyEmailConfig holds SMTP settings
type NotifyEmailConfig struct {
	Enabled     bool     `yaml:"enabled"`
	SMTPHost    string   `yaml:"smtp_host"`
	SMTPPort    int      `yaml:"smtp_port"`
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password"`
	From        string   `yaml:"from"`
	To          []string `yaml:"to"`
	EventTypes  []string `yaml:"events"`
	MinPriority int      `yaml:"min_priority"`
}
