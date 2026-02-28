package tools

import (
	"fmt"

	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type messageArgs struct {
	Content string `json:"content" jsonschema:"Message content to send"`
	Channel string `json:"channel,omitempty" jsonschema:"Target channel (defaults to current)"`
	ChatID  string `json:"chat_id,omitempty" jsonschema:"Target chat ID (defaults to current)"`
}

type messageResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func newMessage(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "message",
		Description: "Send a message to a user on a chat channel.",
	}, func(ctx tool.Context, args messageArgs) (messageResult, error) {
		if args.Content == "" {
			return messageResult{Error: "content is required"}, nil
		}

		ch := args.Channel
		chatID := args.ChatID

		// Fall back to current session context
		if ch == "" {
			ch = stateStr(ctx, "channel")
		}
		if chatID == "" {
			chatID = stateStr(ctx, "chat_id")
		}

		if ch == "" {
			return messageResult{Error: "no target channel"}, nil
		}

		msg := types.OutboundMessage{
			Channel: ch,
			ChatID:  chatID,
			Content: args.Content,
		}
		if err := deps.Bus.PublishOutbound(ctx, msg); err != nil {
			return messageResult{Error: fmt.Sprintf("send failed: %v", err)}, nil
		}
		return messageResult{Result: "message sent"}, nil
	})
	return t
}
