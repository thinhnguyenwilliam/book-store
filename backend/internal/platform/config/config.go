package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Gateway  GatewayConfig  `mapstructure:"gateway"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Redis    RedisConfig    `mapstructure:"redis"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Outbox   OutboxConfig   `mapstructure:"outbox"`
	Shutdown ShutdownConfig `mapstructure:"shutdown"`
}

type GatewayConfig struct {
	HTTPAddress string `mapstructure:"http_address"`
}

type GRPCConfig struct {
	AuthAddress       string `mapstructure:"auth_address"`
	UserAddress       string `mapstructure:"user_address"`
	BookAddress       string `mapstructure:"book_address"`
	AuthListenAddress string `mapstructure:"auth_listen_address"`
	UserListenAddress string `mapstructure:"user_listen_address"`
	BookListenAddress string `mapstructure:"book_listen_address"`
}

type PostgresConfig struct {
	URL string `mapstructure:"url"`
}

type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"`
	JWTIssuer string `mapstructure:"jwt_issuer"`
	JWTTTL    string `mapstructure:"jwt_ttl"`
}

type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
}

type RabbitMQConfig struct {
	URL                         string `mapstructure:"url"`
	Exchange                    string `mapstructure:"exchange"`
	UserProfileQueue            string `mapstructure:"user_profile_queue"`
	AccountRegisteredRoutingKey string `mapstructure:"account_registered_routing_key"`
	ConsumerName                string `mapstructure:"consumer_name"`
	ConsumerConcurrency         int    `mapstructure:"consumer_concurrency"`
	Prefetch                    int    `mapstructure:"prefetch"`
}

type OutboxConfig struct {
	PollInterval string `mapstructure:"outbox_poll_interval"`
}

type ShutdownConfig struct {
	Timeout string `mapstructure:"timeout"`
}

func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
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
		"gateway.http_address":                    c.Gateway.HTTPAddress,
		"grpc.auth_address":                       c.GRPC.AuthAddress,
		"grpc.user_address":                       c.GRPC.UserAddress,
		"grpc.book_address":                       c.GRPC.BookAddress,
		"grpc.auth_listen_address":                c.GRPC.AuthListenAddress,
		"grpc.user_listen_address":                c.GRPC.UserListenAddress,
		"grpc.book_listen_address":                c.GRPC.BookListenAddress,
		"postgres.url":                            c.Postgres.URL,
		"auth.jwt_secret":                         c.Auth.JWTSecret,
		"auth.jwt_issuer":                         c.Auth.JWTIssuer,
		"auth.jwt_ttl":                            c.Auth.JWTTTL,
		"redis.address":                           c.Redis.Address,
		"rabbitmq.url":                            c.RabbitMQ.URL,
		"rabbitmq.exchange":                       c.RabbitMQ.Exchange,
		"rabbitmq.user_profile_queue":             c.RabbitMQ.UserProfileQueue,
		"rabbitmq.account_registered_routing_key": c.RabbitMQ.AccountRegisteredRoutingKey,
		"rabbitmq.consumer_name":                  c.RabbitMQ.ConsumerName,
		"outbox.outbox_poll_interval":             c.Outbox.PollInterval,
		"shutdown.timeout":                        c.Shutdown.Timeout,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("auth.jwt_secret must contain at least 32 characters")
	}
	if c.RabbitMQ.ConsumerConcurrency < 1 {
		return fmt.Errorf("rabbitmq.consumer_concurrency must be greater than zero")
	}
	if c.RabbitMQ.Prefetch < 1 {
		return fmt.Errorf("rabbitmq.prefetch must be greater than zero")
	}
	shutdownTimeout, err := time.ParseDuration(c.Shutdown.Timeout)
	if err != nil || shutdownTimeout <= 0 {
		return fmt.Errorf("shutdown.timeout must be a positive duration")
	}
	return nil
}
