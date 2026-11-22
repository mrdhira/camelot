package app_test

import (
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/src/app"
)

func TestStageTone(t *testing.T) {
	tests := []struct {
		input    taskmodel.Status
		expected app.Tone
	}{
		{input: taskmodel.StatusBacklog, expected: app.ToneNeutral},
		{input: taskmodel.StatusTodo, expected: app.ToneNeutral},
		{input: taskmodel.StatusPlan, expected: app.ToneActive},
		{input: taskmodel.StatusWIP, expected: app.ToneActive},
		{input: taskmodel.StatusQA, expected: app.ToneActive},
		{input: taskmodel.StatusReview, expected: app.ToneYou},
		{input: taskmodel.StatusDone, expected: app.ToneDone},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := app.StageTone(tt.input); got != tt.expected {
				t.Errorf("StageTone(%s) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
	if len(tests) != len(taskmodel.Statuses) {
		t.Errorf("%d stages tested, %d exist", len(tests), len(taskmodel.Statuses))
	}
}

func TestDocTone(t *testing.T) {
	tests := []struct {
		input    docmodel.Status
		expected app.Tone
	}{
		{input: docmodel.StatusDraft, expected: app.ToneNeutral},
		{input: docmodel.StatusInReview, expected: app.ToneYou},
		{input: docmodel.StatusApproved, expected: app.ToneDone},
		{input: docmodel.StatusSuperseded, expected: app.ToneNeutral},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := app.DocTone(tt.input); got != tt.expected {
				t.Errorf("DocTone(%s) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
	if len(tests) != len(docmodel.Statuses) {
		t.Errorf("%d statuses tested, %d exist", len(tests), len(docmodel.Statuses))
	}
}
