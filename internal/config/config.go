package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/samber/do/v2"
)

type Config struct {
	Port                 int    `env:"PORT" envDefault:"80"`
	LogLevel             string `env:"LOG_LEVEL" envDefault:"info"`
	DatabaseURL          string `env:"DATABASE_URL"`
	LLMServerURL         string `env:"LLM_SERVER_URL"`
	EmbeddingServerURL   string `env:"EMBEDDING_SERVER_URL"`
	EmbeddingDimension   int    `env:"EMBEDDING_DIMENSION" envDefault:"768"`
	ChunkSize            int    `env:"CHUNK_SIZE" envDefault:"1000"`
	ChunkOverlap         int    `env:"CHUNK_OVERLAP" envDefault:"200"`
	TopK                 int    `env:"TOP_K" envDefault:"5"`
	SystemPromptTemplate string `env:"SYSTEM_PROMPT_TEMPLATE"`
}

func New(di do.Injector) (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
