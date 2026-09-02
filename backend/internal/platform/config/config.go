package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
)

type Config struct {
	Gateway       GatewayConfig       `mapstructure:"gateway"`
	GRPC          GRPCConfig          `mapstructure:"grpc"`
	Postgres      PostgresConfig      `mapstructure:"postgres"`
	Auth          AuthConfig          `mapstructure:"auth"`
	Payment       PaymentConfig       `mapstructure:"payment"`
	Notification  NotificationConfig  `mapstructure:"notification"`
	Chat          ChatConfig          `mapstructure:"chat"`
	Commerce      CommerceConfig      `mapstructure:"commerce"`
	Redis         RedisConfig         `mapstructure:"redis"`
	RabbitMQ      RabbitMQConfig      `mapstructure:"rabbitmq"`
	Kafka         KafkaConfig         `mapstructure:"kafka"`
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
	Outbox        OutboxConfig        `mapstructure:"outbox"`
	Shutdown      ShutdownConfig      `mapstructure:"shutdown"`
	Logging       logger.Config       `mapstructure:"logging"`
}

type GatewayConfig struct {
	HTTPAddress           string   `mapstructure:"http_address"`
	AllowedOrigins        []string `mapstructure:"allowed_origins"`
	RefreshCookieName     string   `mapstructure:"refresh_cookie_name"`
	RefreshCookieSecure   bool     `mapstructure:"refresh_cookie_secure"`
	RefreshCookieSameSite string   `mapstructure:"refresh_cookie_same_site"`
	RequestTimeout        string   `mapstructure:"request_timeout"`
	PerformanceTarget     string   `mapstructure:"performance_target"`
	ReadHeaderTimeout     string   `mapstructure:"read_header_timeout"`
	ReadTimeout           string   `mapstructure:"read_timeout"`
	WriteTimeout          string   `mapstructure:"write_timeout"`
	IdleTimeout           string   `mapstructure:"idle_timeout"`
	GraphQLBodyLimit      string   `mapstructure:"graphql_body_limit"`
	GraphQLMaxComplexity  int      `mapstructure:"graphql_max_complexity"`
	GraphQLMaxDepth       int      `mapstructure:"graphql_max_depth"`
	GraphQLParserTokens   int      `mapstructure:"graphql_parser_tokens"`
	GraphQLIntrospection  bool     `mapstructure:"graphql_introspection"`
}

type GRPCConfig struct {
	AuthAddress               string `mapstructure:"auth_address"`
	UserAddress               string `mapstructure:"user_address"`
	BookAddress               string `mapstructure:"book_address"`
	OrderAddress              string `mapstructure:"order_address"`
	PaymentAddress            string `mapstructure:"payment_address"`
	NotificationAddress       string `mapstructure:"notification_address"`
	CommentAddress            string `mapstructure:"comment_address"`
	ChatAddress               string `mapstructure:"chat_address"`
	AnalyticsAddress          string `mapstructure:"analytics_address"`
	SearchAddress             string `mapstructure:"search_address"`
	AuthListenAddress         string `mapstructure:"auth_listen_address"`
	UserListenAddress         string `mapstructure:"user_listen_address"`
	BookListenAddress         string `mapstructure:"book_listen_address"`
	OrderListenAddress        string `mapstructure:"order_listen_address"`
	PaymentListenAddress      string `mapstructure:"payment_listen_address"`
	NotificationListenAddress string `mapstructure:"notification_listen_address"`
	CommentListenAddress      string `mapstructure:"comment_listen_address"`
	ChatListenAddress         string `mapstructure:"chat_listen_address"`
	AnalyticsListenAddress    string `mapstructure:"analytics_listen_address"`
	SearchListenAddress       string `mapstructure:"search_listen_address"`
	CallTimeout               string `mapstructure:"call_timeout"`
}

type NotificationConfig struct {
	EmailEnabled      bool           `mapstructure:"email_enabled"`
	EmailPollInterval string         `mapstructure:"email_poll_interval"`
	EmailRetryDelay   string         `mapstructure:"email_retry_delay"`
	EmailMaxAttempts  int            `mapstructure:"email_max_attempts"`
	EmailBatchSize    int            `mapstructure:"email_batch_size"`
	PushEnabled       bool           `mapstructure:"push_enabled"`
	PushPollInterval  string         `mapstructure:"push_poll_interval"`
	PushRetryDelay    string         `mapstructure:"push_retry_delay"`
	PushMaxAttempts   int            `mapstructure:"push_max_attempts"`
	PushBatchSize     int            `mapstructure:"push_batch_size"`
	SMTP              SMTPConfig     `mapstructure:"smtp"`
	Firebase          FirebaseConfig `mapstructure:"firebase"`
}

type FirebaseConfig struct {
	ProjectID       string `mapstructure:"project_id"`
	CredentialsFile string `mapstructure:"credentials_file"`
	StorefrontURL   string `mapstructure:"storefront_url"`
	AdminURL        string `mapstructure:"admin_url"`
	HTTPTimeout     string `mapstructure:"http_timeout"`
}

type ChatConfig struct {
	WebSocketTicketTTL string `mapstructure:"websocket_ticket_ttl"`
	PresenceTTL        string `mapstructure:"presence_ttl"`
	PingInterval       string `mapstructure:"ping_interval"`
	RedisChannel       string `mapstructure:"redis_channel"`
	MaxMessageBytes    int64  `mapstructure:"max_message_bytes"`
}

type SMTPConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	FromAddress string `mapstructure:"from_address"`
	FromName    string `mapstructure:"from_name"`
	StartTLS    bool   `mapstructure:"start_tls"`
	Timeout     string `mapstructure:"timeout"`
}

type PaymentConfig struct {
	Currency           string      `mapstructure:"currency"`
	PlatformOwnerID    string      `mapstructure:"platform_owner_id"`
	FundingOwnerID     string      `mapstructure:"funding_owner_id"`
	ClearingOwnerID    string      `mapstructure:"clearing_owner_id"`
	DefaultProvider    string      `mapstructure:"default_provider"`
	PlatformFeeBPS     int32       `mapstructure:"platform_fee_bps"`
	ReconcileInterval  string      `mapstructure:"reconcile_interval"`
	ReconcileGrace     string      `mapstructure:"reconcile_grace"`
	ReconcileBatchSize int         `mapstructure:"reconcile_batch_size"`
	VNPay              VNPayConfig `mapstructure:"vnpay"`
}

type VNPayConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	PayURL      string `mapstructure:"pay_url"`
	APIURL      string `mapstructure:"api_url"`
	TMNCode     string `mapstructure:"tmn_code"`
	HashSecret  string `mapstructure:"hash_secret"`
	ReturnURL   string `mapstructure:"return_url"`
	ServerIP    string `mapstructure:"server_ip"`
	TimeZone    string `mapstructure:"timezone"`
	ExpireAfter string `mapstructure:"expire_after"`
	HTTPTimeout string `mapstructure:"http_timeout"`
}

type CommerceConfig struct {
	StockReservationTTL   string `mapstructure:"stock_reservation_ttl"`
	ReconcileInterval     string `mapstructure:"reconcile_interval"`
	ReconcileBatchSize    int    `mapstructure:"reconcile_batch_size"`
	PaymentReconcileGrace string `mapstructure:"payment_reconcile_grace"`
}

type PostgresConfig struct {
	URL                   string `mapstructure:"url"`
	MaxOpenConnections    int    `mapstructure:"max_open_connections"`
	MaxIdleConnections    int    `mapstructure:"max_idle_connections"`
	ConnectionMaxLifetime string `mapstructure:"connection_max_lifetime"`
	ConnectionMaxIdleTime string `mapstructure:"connection_max_idle_time"`
}

type AuthConfig struct {
	JWTSecret            string `mapstructure:"jwt_secret"`
	JWTIssuer            string `mapstructure:"jwt_issuer"`
	AccessTokenTTL       string `mapstructure:"access_token_ttl"`
	RefreshTokenTTL      string `mapstructure:"refresh_token_ttl"`
	GoogleClientID       string `mapstructure:"google_client_id"`
	FacebookAppID        string `mapstructure:"facebook_app_id"`
	FacebookAppSecret    string `mapstructure:"facebook_app_secret"`
	FacebookGraphVersion string `mapstructure:"facebook_graph_version"`
}

type RedisConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Address      string `mapstructure:"address"`
	Password     string `mapstructure:"password"`
	Database     int    `mapstructure:"database"`
	Namespace    string `mapstructure:"namespace"`
	DialTimeout  string `mapstructure:"dial_timeout"`
	ReadTimeout  string `mapstructure:"read_timeout"`
	WriteTimeout string `mapstructure:"write_timeout"`
	PoolSize     int    `mapstructure:"pool_size"`
	BookTTL      string `mapstructure:"book_ttl"`
	CartTTL      string `mapstructure:"cart_ttl"`
	LockTTL      string `mapstructure:"lock_ttl"`
}

type RabbitMQConfig struct {
	URL                          string `mapstructure:"url"`
	Exchange                     string `mapstructure:"exchange"`
	UserProfileQueue             string `mapstructure:"user_profile_queue"`
	AccountRegisteredRoutingKey  string `mapstructure:"account_registered_routing_key"`
	AccountDeletedRoutingKey     string `mapstructure:"account_deleted_routing_key"`
	PaymentEventsQueue           string `mapstructure:"payment_events_queue"`
	PaymentSucceededRoutingKey   string `mapstructure:"payment_succeeded_routing_key"`
	PaymentFailedRoutingKey      string `mapstructure:"payment_failed_routing_key"`
	PaymentRefundedRoutingKey    string `mapstructure:"payment_refunded_routing_key"`
	PaymentConsumerName          string `mapstructure:"payment_consumer_name"`
	NotificationEventsQueue      string `mapstructure:"notification_events_queue"`
	NotificationConsumerName     string `mapstructure:"notification_consumer_name"`
	ChatMessageCreatedRoutingKey string `mapstructure:"chat_message_created_routing_key"`
	ConsumerName                 string `mapstructure:"consumer_name"`
	ConsumerConcurrency          int    `mapstructure:"consumer_concurrency"`
	Prefetch                     int    `mapstructure:"prefetch"`
}

type KafkaConfig struct {
	Enabled                bool     `mapstructure:"enabled"`
	Brokers                []string `mapstructure:"brokers"`
	ClientID               string   `mapstructure:"client_id"`
	OrderEventsTopic       string   `mapstructure:"order_events_topic"`
	OrderEventsDLQTopic    string   `mapstructure:"order_events_dlq_topic"`
	AnalyticsConsumerGroup string   `mapstructure:"analytics_consumer_group"`
	CustomerActivityTopic  string   `mapstructure:"customer_activity_topic"`
	CustomerActivityDLQ    string   `mapstructure:"customer_activity_dlq_topic"`
	ActivityConsumerGroup  string   `mapstructure:"activity_consumer_group"`
	ActivityBufferSize     int      `mapstructure:"activity_buffer_size"`
	CatalogEventsTopic     string   `mapstructure:"catalog_events_topic"`
	CatalogEventsDLQTopic  string   `mapstructure:"catalog_events_dlq_topic"`
	SearchConsumerGroup    string   `mapstructure:"search_consumer_group"`
	ConsumerMaxRetries     int      `mapstructure:"consumer_max_retries"`
	ConsumerRetryBackoff   string   `mapstructure:"consumer_retry_backoff"`
}

type ElasticsearchConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	Addresses        []string `mapstructure:"addresses"`
	Username         string   `mapstructure:"username"`
	Password         string   `mapstructure:"password"`
	IndexAlias       string   `mapstructure:"index_alias"`
	RequestTimeout   string   `mapstructure:"request_timeout"`
	BootstrapReindex bool     `mapstructure:"bootstrap_reindex"`
}

type OutboxConfig struct {
	PollInterval string `mapstructure:"outbox_poll_interval"`
}

type ShutdownConfig struct {
	Timeout string `mapstructure:"timeout"`
}

func Load(path string) (Config, error) {
	return LoadWithOverride(path, "")
}

// LoadWithOverride reads a complete base YAML file and optionally merges a partial secret YAML file.
func LoadWithOverride(path, overridePath string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	if strings.TrimSpace(overridePath) != "" {
		v.SetConfigFile(overridePath)
		if err := v.MergeInConfig(); err != nil {
			return Config{}, fmt.Errorf("merge config override %q: %w", overridePath, err)
		}
	}

	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	required := map[string]string{
		"gateway.http_address":                      c.Gateway.HTTPAddress,
		"gateway.refresh_cookie_name":               c.Gateway.RefreshCookieName,
		"gateway.refresh_cookie_same_site":          c.Gateway.RefreshCookieSameSite,
		"gateway.request_timeout":                   c.Gateway.RequestTimeout,
		"gateway.performance_target":                c.Gateway.PerformanceTarget,
		"gateway.read_header_timeout":               c.Gateway.ReadHeaderTimeout,
		"gateway.read_timeout":                      c.Gateway.ReadTimeout,
		"gateway.write_timeout":                     c.Gateway.WriteTimeout,
		"gateway.idle_timeout":                      c.Gateway.IdleTimeout,
		"gateway.graphql_body_limit":                c.Gateway.GraphQLBodyLimit,
		"grpc.auth_address":                         c.GRPC.AuthAddress,
		"grpc.user_address":                         c.GRPC.UserAddress,
		"grpc.book_address":                         c.GRPC.BookAddress,
		"grpc.order_address":                        c.GRPC.OrderAddress,
		"grpc.payment_address":                      c.GRPC.PaymentAddress,
		"grpc.notification_address":                 c.GRPC.NotificationAddress,
		"grpc.comment_address":                      c.GRPC.CommentAddress,
		"grpc.chat_address":                         c.GRPC.ChatAddress,
		"grpc.analytics_address":                    c.GRPC.AnalyticsAddress,
		"grpc.search_address":                       c.GRPC.SearchAddress,
		"grpc.auth_listen_address":                  c.GRPC.AuthListenAddress,
		"grpc.user_listen_address":                  c.GRPC.UserListenAddress,
		"grpc.book_listen_address":                  c.GRPC.BookListenAddress,
		"grpc.order_listen_address":                 c.GRPC.OrderListenAddress,
		"grpc.payment_listen_address":               c.GRPC.PaymentListenAddress,
		"grpc.notification_listen_address":          c.GRPC.NotificationListenAddress,
		"grpc.comment_listen_address":               c.GRPC.CommentListenAddress,
		"grpc.chat_listen_address":                  c.GRPC.ChatListenAddress,
		"grpc.analytics_listen_address":             c.GRPC.AnalyticsListenAddress,
		"grpc.search_listen_address":                c.GRPC.SearchListenAddress,
		"grpc.call_timeout":                         c.GRPC.CallTimeout,
		"postgres.url":                              c.Postgres.URL,
		"postgres.connection_max_lifetime":          c.Postgres.ConnectionMaxLifetime,
		"postgres.connection_max_idle_time":         c.Postgres.ConnectionMaxIdleTime,
		"auth.jwt_secret":                           c.Auth.JWTSecret,
		"auth.jwt_issuer":                           c.Auth.JWTIssuer,
		"auth.access_token_ttl":                     c.Auth.AccessTokenTTL,
		"auth.refresh_token_ttl":                    c.Auth.RefreshTokenTTL,
		"payment.currency":                          c.Payment.Currency,
		"payment.platform_owner_id":                 c.Payment.PlatformOwnerID,
		"payment.funding_owner_id":                  c.Payment.FundingOwnerID,
		"payment.clearing_owner_id":                 c.Payment.ClearingOwnerID,
		"payment.default_provider":                  c.Payment.DefaultProvider,
		"payment.reconcile_interval":                c.Payment.ReconcileInterval,
		"payment.reconcile_grace":                   c.Payment.ReconcileGrace,
		"notification.smtp.from_address":            c.Notification.SMTP.FromAddress,
		"notification.smtp.from_name":               c.Notification.SMTP.FromName,
		"notification.smtp.timeout":                 c.Notification.SMTP.Timeout,
		"notification.email_poll_interval":          c.Notification.EmailPollInterval,
		"notification.email_retry_delay":            c.Notification.EmailRetryDelay,
		"notification.push_poll_interval":           c.Notification.PushPollInterval,
		"notification.push_retry_delay":             c.Notification.PushRetryDelay,
		"notification.firebase.http_timeout":        c.Notification.Firebase.HTTPTimeout,
		"chat.websocket_ticket_ttl":                 c.Chat.WebSocketTicketTTL,
		"chat.presence_ttl":                         c.Chat.PresenceTTL,
		"chat.ping_interval":                        c.Chat.PingInterval,
		"chat.redis_channel":                        c.Chat.RedisChannel,
		"commerce.stock_reservation_ttl":            c.Commerce.StockReservationTTL,
		"commerce.reconcile_interval":               c.Commerce.ReconcileInterval,
		"commerce.payment_reconcile_grace":          c.Commerce.PaymentReconcileGrace,
		"redis.address":                             c.Redis.Address,
		"redis.namespace":                           c.Redis.Namespace,
		"redis.dial_timeout":                        c.Redis.DialTimeout,
		"redis.read_timeout":                        c.Redis.ReadTimeout,
		"redis.write_timeout":                       c.Redis.WriteTimeout,
		"redis.book_ttl":                            c.Redis.BookTTL,
		"redis.cart_ttl":                            c.Redis.CartTTL,
		"redis.lock_ttl":                            c.Redis.LockTTL,
		"rabbitmq.url":                              c.RabbitMQ.URL,
		"rabbitmq.exchange":                         c.RabbitMQ.Exchange,
		"rabbitmq.user_profile_queue":               c.RabbitMQ.UserProfileQueue,
		"rabbitmq.account_registered_routing_key":   c.RabbitMQ.AccountRegisteredRoutingKey,
		"rabbitmq.account_deleted_routing_key":      c.RabbitMQ.AccountDeletedRoutingKey,
		"rabbitmq.payment_events_queue":             c.RabbitMQ.PaymentEventsQueue,
		"rabbitmq.payment_succeeded_routing_key":    c.RabbitMQ.PaymentSucceededRoutingKey,
		"rabbitmq.payment_failed_routing_key":       c.RabbitMQ.PaymentFailedRoutingKey,
		"rabbitmq.payment_refunded_routing_key":     c.RabbitMQ.PaymentRefundedRoutingKey,
		"rabbitmq.payment_consumer_name":            c.RabbitMQ.PaymentConsumerName,
		"rabbitmq.notification_events_queue":        c.RabbitMQ.NotificationEventsQueue,
		"rabbitmq.notification_consumer_name":       c.RabbitMQ.NotificationConsumerName,
		"rabbitmq.chat_message_created_routing_key": c.RabbitMQ.ChatMessageCreatedRoutingKey,
		"rabbitmq.consumer_name":                    c.RabbitMQ.ConsumerName,
		"outbox.outbox_poll_interval":               c.Outbox.PollInterval,
		"shutdown.timeout":                          c.Shutdown.Timeout,
		"logging.directory":                         c.Logging.Directory,
		"logging.level":                             c.Logging.Level,
		"logging.format":                            c.Logging.Format,
		"logging.timezone":                          c.Logging.TimeZone,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("auth.jwt_secret must contain at least 32 characters")
	}
	facebookAppConfigured := strings.TrimSpace(c.Auth.FacebookAppID) != ""
	facebookSecretConfigured := strings.TrimSpace(c.Auth.FacebookAppSecret) != ""
	if facebookAppConfigured != facebookSecretConfigured {
		return fmt.Errorf("auth.facebook_app_id and auth.facebook_app_secret must be configured together")
	}
	if facebookAppConfigured && strings.TrimSpace(c.Auth.FacebookGraphVersion) == "" {
		return fmt.Errorf("auth.facebook_graph_version is required when Facebook Login is configured")
	}
	if len(c.Gateway.AllowedOrigins) == 0 {
		return fmt.Errorf("gateway.allowed_origins must contain at least one trusted origin")
	}
	sameSite := strings.ToLower(c.Gateway.RefreshCookieSameSite)
	if sameSite != "strict" && sameSite != "lax" && sameSite != "none" {
		return fmt.Errorf("gateway.refresh_cookie_same_site must be strict, lax, or none")
	}
	if sameSite == "none" && !c.Gateway.RefreshCookieSecure {
		return fmt.Errorf("gateway.refresh_cookie_secure must be true when SameSite is none")
	}
	if c.Gateway.GraphQLMaxComplexity < 1 || c.Gateway.GraphQLMaxDepth < 1 || c.Gateway.GraphQLParserTokens < 1 {
		return fmt.Errorf("gateway GraphQL limits must be positive")
	}
	for key, value := range map[string]string{
		"gateway.request_timeout":            c.Gateway.RequestTimeout,
		"gateway.performance_target":         c.Gateway.PerformanceTarget,
		"gateway.read_header_timeout":        c.Gateway.ReadHeaderTimeout,
		"gateway.read_timeout":               c.Gateway.ReadTimeout,
		"gateway.write_timeout":              c.Gateway.WriteTimeout,
		"gateway.idle_timeout":               c.Gateway.IdleTimeout,
		"postgres.connection_max_lifetime":   c.Postgres.ConnectionMaxLifetime,
		"postgres.connection_max_idle_time":  c.Postgres.ConnectionMaxIdleTime,
		"auth.access_token_ttl":              c.Auth.AccessTokenTTL,
		"auth.refresh_token_ttl":             c.Auth.RefreshTokenTTL,
		"grpc.call_timeout":                  c.GRPC.CallTimeout,
		"commerce.stock_reservation_ttl":     c.Commerce.StockReservationTTL,
		"commerce.reconcile_interval":        c.Commerce.ReconcileInterval,
		"commerce.payment_reconcile_grace":   c.Commerce.PaymentReconcileGrace,
		"redis.dial_timeout":                 c.Redis.DialTimeout,
		"redis.read_timeout":                 c.Redis.ReadTimeout,
		"redis.write_timeout":                c.Redis.WriteTimeout,
		"redis.book_ttl":                     c.Redis.BookTTL,
		"redis.cart_ttl":                     c.Redis.CartTTL,
		"redis.lock_ttl":                     c.Redis.LockTTL,
		"payment.reconcile_interval":         c.Payment.ReconcileInterval,
		"payment.reconcile_grace":            c.Payment.ReconcileGrace,
		"notification.smtp.timeout":          c.Notification.SMTP.Timeout,
		"notification.email_poll_interval":   c.Notification.EmailPollInterval,
		"notification.email_retry_delay":     c.Notification.EmailRetryDelay,
		"notification.push_poll_interval":    c.Notification.PushPollInterval,
		"notification.push_retry_delay":      c.Notification.PushRetryDelay,
		"notification.firebase.http_timeout": c.Notification.Firebase.HTTPTimeout,
		"chat.websocket_ticket_ttl":          c.Chat.WebSocketTicketTTL,
		"chat.presence_ttl":                  c.Chat.PresenceTTL,
		"chat.ping_interval":                 c.Chat.PingInterval,
	} {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return fmt.Errorf("%s must be a positive duration", key)
		}
	}
	if c.Notification.SMTP.Port < 1 || c.Notification.SMTP.Port > 65535 {
		return fmt.Errorf("notification.smtp.port must be between 1 and 65535")
	}
	if c.Chat.MaxMessageBytes < 1024 || c.Chat.MaxMessageBytes > 1<<20 {
		return fmt.Errorf("chat.max_message_bytes must be between 1024 and 1048576")
	}
	if c.Notification.EmailMaxAttempts < 1 || c.Notification.EmailMaxAttempts > 100 {
		return fmt.Errorf("notification.email_max_attempts must be between 1 and 100")
	}
	if c.Notification.EmailBatchSize < 1 || c.Notification.EmailBatchSize > 1000 {
		return fmt.Errorf("notification.email_batch_size must be between 1 and 1000")
	}
	if c.Notification.PushMaxAttempts < 1 || c.Notification.PushMaxAttempts > 100 {
		return fmt.Errorf("notification.push_max_attempts must be between 1 and 100")
	}
	if c.Notification.PushBatchSize < 1 || c.Notification.PushBatchSize > 1000 {
		return fmt.Errorf("notification.push_batch_size must be between 1 and 1000")
	}
	if c.Notification.EmailEnabled && strings.TrimSpace(c.Notification.SMTP.Host) == "" {
		return fmt.Errorf("notification.smtp.host is required when email is enabled")
	}
	if (c.Notification.SMTP.Username == "") != (c.Notification.SMTP.Password == "") {
		return fmt.Errorf("notification.smtp.username and password must be configured together")
	}
	if c.Notification.PushEnabled && strings.TrimSpace(c.Notification.Firebase.ProjectID) == "" {
		return fmt.Errorf("notification.firebase.project_id is required when push is enabled")
	}
	if c.Postgres.MaxOpenConnections < 1 {
		return fmt.Errorf("postgres.max_open_connections must be greater than zero")
	}
	if len(strings.TrimSpace(c.Payment.Currency)) != 3 {
		return fmt.Errorf("payment.currency must be a three-letter currency code")
	}
	if c.Payment.PlatformFeeBPS < 0 || c.Payment.PlatformFeeBPS > 10000 {
		return fmt.Errorf("payment.platform_fee_bps must be between 0 and 10000")
	}
	if c.Payment.ReconcileBatchSize < 1 || c.Payment.ReconcileBatchSize > 1000 {
		return fmt.Errorf("payment.reconcile_batch_size must be between 1 and 1000")
	}
	provider := strings.ToLower(strings.TrimSpace(c.Payment.DefaultProvider))
	if provider != "wallet" && provider != "vnpay" {
		return fmt.Errorf("payment.default_provider must be wallet or vnpay")
	}
	if provider == "vnpay" && !c.Payment.VNPay.Enabled {
		return fmt.Errorf("payment.vnpay must be enabled when it is the default provider")
	}
	if c.Payment.VNPay.Enabled {
		for key, value := range map[string]string{
			"payment.vnpay.pay_url":      c.Payment.VNPay.PayURL,
			"payment.vnpay.api_url":      c.Payment.VNPay.APIURL,
			"payment.vnpay.tmn_code":     c.Payment.VNPay.TMNCode,
			"payment.vnpay.hash_secret":  c.Payment.VNPay.HashSecret,
			"payment.vnpay.return_url":   c.Payment.VNPay.ReturnURL,
			"payment.vnpay.server_ip":    c.Payment.VNPay.ServerIP,
			"payment.vnpay.timezone":     c.Payment.VNPay.TimeZone,
			"payment.vnpay.expire_after": c.Payment.VNPay.ExpireAfter,
			"payment.vnpay.http_timeout": c.Payment.VNPay.HTTPTimeout,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s is required when VNPAY is enabled", key)
			}
		}
		for key, value := range map[string]string{
			"payment.vnpay.expire_after": c.Payment.VNPay.ExpireAfter,
			"payment.vnpay.http_timeout": c.Payment.VNPay.HTTPTimeout,
		} {
			if duration, err := time.ParseDuration(value); err != nil || duration <= 0 {
				return fmt.Errorf("%s must be a positive duration", key)
			}
		}
		if _, err := time.LoadLocation(c.Payment.VNPay.TimeZone); err != nil {
			return fmt.Errorf("payment.vnpay.timezone is invalid: %w", err)
		}
	}
	if c.Commerce.ReconcileBatchSize < 1 || c.Commerce.ReconcileBatchSize > 1000 {
		return fmt.Errorf("commerce.reconcile_batch_size must be between 1 and 1000")
	}
	if c.Postgres.MaxIdleConnections < 0 || c.Postgres.MaxIdleConnections > c.Postgres.MaxOpenConnections {
		return fmt.Errorf("postgres.max_idle_connections must be between zero and max_open_connections")
	}
	if c.Redis.Database < 0 {
		return fmt.Errorf("redis.database must not be negative")
	}
	if c.Redis.PoolSize < 1 {
		return fmt.Errorf("redis.pool_size must be greater than zero")
	}
	if c.RabbitMQ.ConsumerConcurrency < 1 {
		return fmt.Errorf("rabbitmq.consumer_concurrency must be greater than zero")
	}
	if c.RabbitMQ.Prefetch < 1 {
		return fmt.Errorf("rabbitmq.prefetch must be greater than zero")
	}
	if c.Kafka.Enabled {
		if len(c.Kafka.Brokers) == 0 {
			return fmt.Errorf("kafka.brokers must contain at least one broker when Kafka is enabled")
		}
		for key, value := range map[string]string{
			"kafka.client_id":                   c.Kafka.ClientID,
			"kafka.order_events_topic":          c.Kafka.OrderEventsTopic,
			"kafka.order_events_dlq_topic":      c.Kafka.OrderEventsDLQTopic,
			"kafka.analytics_consumer_group":    c.Kafka.AnalyticsConsumerGroup,
			"kafka.customer_activity_topic":     c.Kafka.CustomerActivityTopic,
			"kafka.customer_activity_dlq_topic": c.Kafka.CustomerActivityDLQ,
			"kafka.activity_consumer_group":     c.Kafka.ActivityConsumerGroup,
			"kafka.catalog_events_topic":        c.Kafka.CatalogEventsTopic,
			"kafka.catalog_events_dlq_topic":    c.Kafka.CatalogEventsDLQTopic,
			"kafka.search_consumer_group":       c.Kafka.SearchConsumerGroup,
			"kafka.consumer_retry_backoff":      c.Kafka.ConsumerRetryBackoff,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s is required when Kafka is enabled", key)
			}
		}
		if duration, err := time.ParseDuration(c.Kafka.ConsumerRetryBackoff); err != nil || duration <= 0 {
			return fmt.Errorf("kafka.consumer_retry_backoff must be a positive duration")
		}
		if c.Kafka.ConsumerMaxRetries < 0 || c.Kafka.ConsumerMaxRetries > 20 {
			return fmt.Errorf("kafka.consumer_max_retries must be between 0 and 20")
		}
		if c.Kafka.ActivityBufferSize < 1 || c.Kafka.ActivityBufferSize > 100000 {
			return fmt.Errorf("kafka.activity_buffer_size must be between 1 and 100000")
		}
	}
	if c.Elasticsearch.Enabled {
		if len(c.Elasticsearch.Addresses) == 0 {
			return fmt.Errorf("elasticsearch.addresses must contain at least one address when Elasticsearch is enabled")
		}
		for key, value := range map[string]string{
			"elasticsearch.index_alias":     c.Elasticsearch.IndexAlias,
			"elasticsearch.request_timeout": c.Elasticsearch.RequestTimeout,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s is required when Elasticsearch is enabled", key)
			}
		}
		if duration, err := time.ParseDuration(c.Elasticsearch.RequestTimeout); err != nil || duration <= 0 {
			return fmt.Errorf("elasticsearch.request_timeout must be a positive duration")
		}
		if (c.Elasticsearch.Username == "") != (c.Elasticsearch.Password == "") {
			return fmt.Errorf("elasticsearch.username and password must be configured together")
		}
	}
	shutdownTimeout, err := time.ParseDuration(c.Shutdown.Timeout)
	if err != nil || shutdownTimeout <= 0 {
		return fmt.Errorf("shutdown.timeout must be a positive duration")
	}
	if _, err := time.LoadLocation(c.Logging.TimeZone); err != nil {
		return fmt.Errorf("logging.timezone is invalid: %w", err)
	}
	if format := strings.ToLower(c.Logging.Format); format != "json" && format != "text" {
		return fmt.Errorf("logging.format must be json or text")
	}
	if c.Logging.MaxSizeMB < 1 {
		return fmt.Errorf("logging.max_size_mb must be greater than zero")
	}
	if c.Logging.MaxAgeDays < 0 {
		return fmt.Errorf("logging.max_age_days must not be negative")
	}
	if c.Logging.MaxBackups < 0 {
		return fmt.Errorf("logging.max_backups must not be negative")
	}
	return nil
}
