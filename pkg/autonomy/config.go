package autonomy

type AutonomyConfig struct {
	IdleTrigger IdleTriggerConfig `json:"idle_trigger"`
	Feeds       []FeedConfig      `json:"feeds"`
}

type IdleTriggerConfig struct {
	Enabled        bool `json:"enabled"`
	ThresholdHours int  `json:"threshold_hours"`
}

type FeedConfig struct {
	URL                  string `json:"url"`
	CheckIntervalMinutes int    `json:"check_interval_minutes"`
	NotifyChannel        string `json:"notify_channel"`
	NotifyChatID         string `json:"notify_chat_id"`
}

type FeedItem struct {
	Title       string
	Link        string
	Description string
	Published   string
}
