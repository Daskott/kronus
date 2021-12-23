package shared

type ServerConfig struct {
	Sqlite SqliteConfig `mapstructure:"sqlite" validate:"required"`
	Kronus KronusConfig `mapstructure:"kronus" validate:"required"`
	Google GoogleConfig `mapstructure:"google"`
	Twilio TwilioConfig `mapstructure:"twilio"`
}

type SqliteConfig struct {
	PassPhrase string `mapstructure:"passPhrase" validate:"required"`
}

type KronusConfig struct {
	PrivateKeyPem string         `mapstructure:"privateKeyPem" validate:"required"`
	Cron          CronConfig     `mapstructure:"cron" validate:"required"`
	Listener      ListenerConfig `mapstructure:"listener" validate:"required"`
	PublicUrl     string         `mapstructure:"publicUrl" validate:"required"`
}

type GoogleConfig struct {
	ApplicationCredentials string        `mapstructure:"applicationCredentials"`
	Storage                StorageConfig `mapstructure:"storage"`
}

type CronConfig struct {
	TimeZone string `mapstructure:"timeZone" validate:"required"`
}

type ListenerConfig struct {
	Port int `mapstructure:"port" validate:"required"`
}

type StorageConfig struct {
	Bucket                    string      `mapstructure:"bucket" validate:"required_with=EnableSqliteBackupAndSync"`
	Prefix                    string      `mapstructure:"prefix" validate:"required_with=EnableSqliteBackupAndSync"`
	SqliteBackupSchedule      string      `mapstructure:"sqliteBackupSchedule" validate:"required_with=EnableSqliteBackupAndSync"`
	EnableSqliteBackupAndSync interface{} `mapstructure:"enableSqliteBackupAndSync" validate:"omitempty,bool"`
}

type TwilioConfig struct {
	AccountSid          string `mapstructure:"accountSid" validate:"required"`
	AuthToken           string `mapstructure:"authToken" validate:"required"`
	MessagingServiceSid string `mapstructure:"messagingServiceSid" validate:"required"`
}
