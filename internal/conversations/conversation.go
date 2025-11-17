package conversations

import (
	"context"

	"github.com/lechgu/tichy/internal/responders"
	"github.com/openai/openai-go"
	"github.com/samber/do/v2"
)

type Conversation struct {
	responder *responders.Responder
	history   []openai.ChatCompletionMessageParamUnion
}

func New(i do.Injector) (*Conversation, error) {
	responder, err := do.Invoke[*responders.Responder](i)
	if err != nil {
		return nil, err
	}

	return &Conversation{
		responder: responder,
		history:   make([]openai.ChatCompletionMessageParamUnion, 0),
	}, nil
}

func (c *Conversation) Send(ctx context.Context, query string) (string, error) {
	messages := append(c.history, openai.UserMessage(query))

	response, err := c.responder.Respond(ctx, messages, query)
	if err != nil {
		return "", err
	}

	c.history = append(c.history, openai.UserMessage(query))
	c.history = append(c.history, openai.AssistantMessage(response))

	return response, nil
}
