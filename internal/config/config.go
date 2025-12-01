package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/samber/do/v2"
)

type Config struct {
	Port                 int      `env:"PORT" envDefault:"80"`
	LogLevel             string   `env:"LOG_LEVEL" envDefault:"info"`
	DatabaseURL          string   `env:"DATABASE_URL"`
	LLMServerURL         string   `env:"LLM_SERVER_URL"`
	EmbeddingServerURL   string   `env:"EMBEDDING_SERVER_URL"`
	EmbeddingDimension   int      `env:"EMBEDDING_DIMENSION" envDefault:"768"`
	ChunkSize            int      `env:"CHUNK_SIZE" envDefault:"1000"`
	ChunkOverlap         int      `env:"CHUNK_OVERLAP" envDefault:"200"`
	TopK                 int      `env:"TOP_K" envDefault:"5"`
	SystemPromptTemplate string   `env:"SYSTEM_PROMPT_TEMPLATE"`
	VectorBackend        string   `env:"VECTORDB_BACKEND"`
	FileExtensions       []string `env:"FILE_EXTENSIONS"`
	Qdrant               Qdrant
}

type Qdrant struct {
	Collection       string `env:"QDRANT_COLLECTION"`
	CollectionSize   uint64 `env:"QDRANT_COLLECTION_SIZE"`
	Host             string `env:"QDRANT_HOST"`
	Port             int    `env:"QDRANT_PORT"`
	APIKey           string `env:"QDRANT_API_KEY"`
	UseTLS           bool   `env:"QDRANT_USE_TLS"`
	Recreate         bool   `env:"QDRANT_RECREATE_COLLECTION"`
	PoolSize         uint   `env:"QDRANT_POOL_SIZE"`
	KeepAliveTime    int    `env:"QDRANT_KEEP_ALIVE_TIME"`
	KeepAliveTimeout uint   `env:"QDRANT_KEEP_ALIVE_TIMEOUT"`
}

func New(di do.Injector) (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
