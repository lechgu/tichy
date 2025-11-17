package chat

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/lechgu/tichy/internal/conversations"
	"github.com/lechgu/tichy/internal/injectors"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

var (
	markdown bool
)

var Cmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	RunE:  doChat,
}

func init() {
	Cmd.Flags().BoolVar(&markdown, "markdown", false, "Enable markdown rendering")
}

func doChat(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	conversation, err := do.Invoke[*conversations.Conversation](injectors.Default)
	if err != nil {
		return err
	}

	return runREPL(ctx, cmd, conversation)
}

func runREPL(ctx context.Context, cmd *cobra.Command, conversation *conversations.Conversation) error {
	scanner := bufio.NewScanner(os.Stdin)

	var renderer *glamour.TermRenderer
	if markdown {
		var err error
		renderer, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(100),
		)
		if err != nil {
			return fmt.Errorf("failed to create markdown renderer: %w", err)
		}
	}

	cmd.Println("Chat session started. Type 'exit' or 'quit' to end.")
	cmd.Println()

	for {
		cmd.Print("> ")

		if !scanner.Scan() {
			break
		}

		query := strings.TrimSpace(scanner.Text())

		if query == "" {
			continue
		}

		if query == "exit" || query == "quit" {
			cmd.Println("Goodbye!")
			break
		}

		response, err := conversation.Send(ctx, query)
		if err != nil {
			cmd.Printf("Error: %v\n", err)
			continue
		}

		if renderer != nil {
			rendered, err := renderer.Render(response)
			if err != nil {
				cmd.Println(response)
			} else {
				cmd.Print(rendered)
			}
		} else {
			cmd.Println(response)
		}
		cmd.Println()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}
