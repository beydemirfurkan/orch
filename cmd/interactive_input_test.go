package cmd

import (
	"strings"
	"testing"
)

func TestPrepareInteractiveDispatchPlainChat(t *testing.T) {
	dispatch, err := prepareInteractiveDispatch("selam")
	if err != nil {
		t.Fatalf("prepare dispatch: %v", err)
	}
	if len(dispatch.Args) != 2 || dispatch.Args[0] != "chat" || dispatch.Args[1] != "selam" {
		t.Fatalf("unexpected args: %+v", dispatch.Args)
	}
	if dispatch.DisplayInput != "selam" {
		t.Fatalf("unexpected display input: %q", dispatch.DisplayInput)
	}
	if dispatch.InputNote != "" {
		t.Fatalf("expected no input note, got %q", dispatch.InputNote)
	}
}

func TestPrepareInteractiveDispatchRunCommand(t *testing.T) {
	dispatch, err := prepareInteractiveDispatch("/run fix auth bug")
	if err != nil {
		t.Fatalf("prepare dispatch: %v", err)
	}
	if len(dispatch.Args) != 2 || dispatch.Args[0] != "run" || dispatch.Args[1] != "fix auth bug" {
		t.Fatalf("unexpected args: %+v", dispatch.Args)
	}
}

func TestPrepareInteractiveDispatchQuickTransform(t *testing.T) {
	dispatch, err := prepareInteractiveDispatch("?quick explain confidence policy")
	if err != nil {
		t.Fatalf("prepare dispatch: %v", err)
	}
	if len(dispatch.Args) != 2 || dispatch.Args[0] != "chat" {
		t.Fatalf("unexpected args: %+v", dispatch.Args)
	}
	if !strings.Contains(dispatch.Args[1], "Respond briefly and practically") {
		t.Fatalf("expected transformed prompt, got %q", dispatch.Args[1])
	}
	if dispatch.InputNote == "" {
		t.Fatalf("expected input transform note")
	}
}

func TestPrepareInteractiveDispatchQuickRequiresMessage(t *testing.T) {
	_, err := prepareInteractiveDispatch("?quick")
	if err == nil {
		t.Fatalf("expected error for empty ?quick input")
	}
}
