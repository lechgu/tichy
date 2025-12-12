package servers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lechgu/tichy/internal/auth"
	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/lechgu/tichy/internal/responders"
	"github.com/openai/openai-go"
	"github.com/samber/do/v2"
	"github.com/sirupsen/logrus"
)

type AuthzServer struct {
	http.Server
	cfg       *config.Config
	responder *responders.Responder
	logger    *logrus.Logger
	router    *gin.Engine
	jwtSecret []byte
}

func NewAuthzServer(i do.Injector, jwtSecret string) (*AuthzServer, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, err
	}

	responder, err := do.Invoke[*responders.Responder](i)
	if err != nil {
		return nil, err
	}

	logger, err := do.Invoke[*logrus.Logger](i)
	if err != nil {
		return nil, err
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &AuthzServer{
		cfg:       cfg,
		responder: responder,
		logger:    logger,
		router:    router,
		jwtSecret: []byte(jwtSecret),
	}

	s.setupRoutes()

	return s, nil
}

func (s *AuthzServer) setupRoutes() {
	s.router.GET("/healthz", s.handleHealth)
	v1 := s.router.Group("/v1", AuthMiddleware(s.jwtSecret))
	{
		v1.POST("/chat/completions", s.handleChatCompletions)
		v1.GET("/user/info", s.handleUserInfo)
	}
}

func (s *AuthzServer) Run(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.Addr = addr
	s.Handler = s.router

	errors := make(chan error, 1)
	go func() {
		s.logger.Infof("Starting server on %s", addr)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errors <- err
		}
	}()

	select {
	case err := <-errors:
		return err
	case <-ctx.Done():
		s.logger.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

func (s *AuthzServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *AuthzServer) handleUserInfo(c *gin.Context) {
	user, ok := auth.UserFromContext(c.Request.Context())
	if ok {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "user": user})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"status": "user info is not found in JWT token"})
}

func (s *AuthzServer) handleChatCompletions(c *gin.Context) {
	var req models.ChatCompletionRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "messages cannot be empty"})
		return
	}

	var lastUserMessage string
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			openaiMessages = append(openaiMessages, openai.UserMessage(msg.Content))
			lastUserMessage = msg.Content
		case "assistant":
			openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Content))
		case "system":
			// Skip system messages - we'll add our own with RAG context
		}
	}

	if lastUserMessage == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "no user message found"})
		return
	}

	// extract from http request user's token and create specific context with
	// user based information

	response, err := s.responder.Respond(c.Request.Context(), openaiMessages, lastUserMessage)
	if err != nil {
		s.logger.Errorf("Chat completion error: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate response"})
		return
	}

	c.JSON(http.StatusOK, models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []models.Choice{
			{
				Index: 0,
				Message: models.Message{
					Role:    "assistant",
					Content: response,
				},
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{
			PromptTokens:     len(lastUserMessage) / 4,
			CompletionTokens: len(response) / 4,
			TotalTokens:      (len(lastUserMessage) + len(response)) / 4,
		},
	})
}
