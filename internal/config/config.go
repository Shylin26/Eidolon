package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	Inference InferenceConfig `mapstructure:"inference"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Server    ServerConfig    `mapstructure:"server"`
}

type KafkaConfig struct {
	BootstrapServers string `mapstructure:"bootstrap_servers"`
	GroupID          string `mapstructure:"group_id"`
}

type InferenceConfig struct {
	SocketPath  string  `mapstructure:"socket_path"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
	TopP        float64 `mapstructure:"top_p"`
}

type StorageConfig struct {
	SQLitePath string `mapstructure:"sqlite_path"`
	VectorDSN  string `mapstructure:"vector_dsn"`
}

type ServerConfig struct {
	GRPCPort int `mapstructure:"grpc_port"`
	HTTPPort int `mapstructure:"http_port"`
}

func Load() (*Config, error) {
	viper.SetConfigName("eidolon")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/eidolon")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.SetDefault("kafka.bootstrap_servers", "localhost:9092")
	viper.SetDefault("kafka.group_id", "eidolon")
	viper.SetDefault("inference.socket_path", "/tmp/eidolon.sock")
	viper.SetDefault("inference.max_tokens", 256)
	viper.SetDefault("inference.temperature", 0.2)
	viper.SetDefault("inference.top_p", 0.95)
	viper.SetDefault("storage.sqlite_path", "$HOME/eidolon/data/eidolon.db")
	viper.SetDefault("server.grpc_port", 50051)
	viper.SetDefault("server.http_port", 8080)

	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
