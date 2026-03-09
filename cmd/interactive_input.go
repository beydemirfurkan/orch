package cmd

import (
	"fmt"
	"strings"
)

type interactiveDispatch struct {
	Args         []string
	DisplayInput string
	InputNote    string
}

func prepareInteractiveDispatch(input string) (interactiveDispatch, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return interactiveDispatch{}, fmt.Errorf("empty input")
	}

	if strings.HasPrefix(trimmed, "?quick") {
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "?quick"))
		if payload == "" {
			return interactiveDispatch{}, fmt.Errorf("?quick requires a message")
		}
		return interactiveDispatch{
			Args:         []string{"chat", buildQuickChatPrompt(payload)},
			DisplayInput: trimmed,
			InputNote:    "Local input transform applied: concise chat mode.",
		}, nil
	}

	args, err := parseInteractiveInput(trimmed)
	if err != nil {
		return interactiveDispatch{}, err
	}
	return interactiveDispatch{
		Args:         args,
		DisplayInput: trimmed,
	}, nil
}

func buildQuickChatPrompt(message string) string {
	message = strings.TrimSpace(message)
	return "Respond briefly and practically. Keep the answer tight, actionable, and under 6 lines when possible.\n\nUser request: " + message
}
