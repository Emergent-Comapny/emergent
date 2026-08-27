package chat

import "testing"

func TestQuestionEventFromAskUser(t *testing.T) {
	t.Run("maps full payload", func(t *testing.T) {
		input := map[string]any{
			"question":         "Which theme?",
			"interaction_type": "buttons",
			"placeholder":      "pick one",
			"max_length":       120.0,
			"options": []any{
				map[string]any{"label": "Dark", "value": "dark", "description": "dark theme"},
				map[string]any{"label": "Light", "value": "light"},
			},
		}
		output := map[string]any{
			"question_id": "q-123",
			"status":      "pausing",
		}

		got, ok := questionEventFromAskUser(input, output)
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if got.Type != "question" {
			t.Errorf("Type = %q, want %q", got.Type, "question")
		}
		if got.QuestionID != "q-123" {
			t.Errorf("QuestionID = %q, want q-123", got.QuestionID)
		}
		if got.Question != "Which theme?" {
			t.Errorf("Question = %q", got.Question)
		}
		if got.InteractionType != "buttons" {
			t.Errorf("InteractionType = %q", got.InteractionType)
		}
		if got.Placeholder != "pick one" {
			t.Errorf("Placeholder = %q", got.Placeholder)
		}
		if got.MaxLength != 120 {
			t.Errorf("MaxLength = %d, want 120", got.MaxLength)
		}
		if len(got.Options) != 2 {
			t.Fatalf("len(Options) = %d, want 2", len(got.Options))
		}
		if got.Options[0].Label != "Dark" || got.Options[0].Value != "dark" || got.Options[0].Description != "dark theme" {
			t.Errorf("Options[0] = %+v", got.Options[0])
		}
		if got.Options[1].Label != "Light" || got.Options[1].Value != "light" {
			t.Errorf("Options[1] = %+v", got.Options[1])
		}
		if got.Options[1].Description != "" {
			t.Errorf("Options[1].Description = %q, want empty", got.Options[1].Description)
		}
	})

	t.Run("missing question_id returns false", func(t *testing.T) {
		if _, ok := questionEventFromAskUser(map[string]any{"question": "q"}, map[string]any{"status": "pausing"}); ok {
			t.Fatalf("expected ok=false when output has no question_id")
		}
	})

	t.Run("nil output returns false", func(t *testing.T) {
		if _, ok := questionEventFromAskUser(map[string]any{"question": "q"}, nil); ok {
			t.Fatalf("expected ok=false for nil output")
		}
	})

	t.Run("empty interaction_type defaults to buttons", func(t *testing.T) {
		got, ok := questionEventFromAskUser(
			map[string]any{"question": "q", "interaction_type": ""},
			map[string]any{"question_id": "q-1"},
		)
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if got.InteractionType != "buttons" {
			t.Errorf("InteractionType = %q, want buttons", got.InteractionType)
		}
	})

	t.Run("max_length int variant", func(t *testing.T) {
		got, _ := questionEventFromAskUser(
			map[string]any{"max_length": 42},
			map[string]any{"question_id": "q-2"},
		)
		if got.MaxLength != 42 {
			t.Errorf("MaxLength = %d, want 42", got.MaxLength)
		}
	})

	t.Run("skips non-map options", func(t *testing.T) {
		got, ok := questionEventFromAskUser(
			map[string]any{"options": []any{"bad", map[string]any{"label": "A", "value": "a"}}},
			map[string]any{"question_id": "q-3"},
		)
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if len(got.Options) != 1 || got.Options[0].Value != "a" {
			t.Errorf("Options = %+v, want single option value 'a'", got.Options)
		}
	})

	t.Run("nil input still yields ok with question_id only", func(t *testing.T) {
		got, ok := questionEventFromAskUser(nil, map[string]any{"question_id": "q-4"})
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if got.QuestionID != "q-4" || got.Question != "" || len(got.Options) != 0 {
			t.Errorf("unexpected result: %+v", got)
		}
	})
}
