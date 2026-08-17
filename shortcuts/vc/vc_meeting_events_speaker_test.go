// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package vc

import (
	"reflect"
	"testing"
)

func TestBuildMeetingEventTimeline_DisambiguatesTranscriptSpeakers(t *testing.T) {
	event := map[string]interface{}{
		"event_type": "transcript_received",
		"payload": map[string]interface{}{
			"transcript_received_items": []interface{}{
				map[string]interface{}{"speaker": map[string]interface{}{"id": "u1", "user_name": "Alice"}, "text": "one"},
				map[string]interface{}{"speaker": map[string]interface{}{"id": "u2", "user_name": "Alice"}, "text": "two"},
				map[string]interface{}{"speaker": map[string]interface{}{"id": "u1", "user_name": "Alice"}, "text": "three"},
				map[string]interface{}{"speaker": map[string]interface{}{"id": "u3", "user_name": "Bob"}, "text": "four"},
				map[string]interface{}{"speaker": map[string]interface{}{"id": "u4"}, "text": "five"},
			},
		},
	}

	timeline := buildMeetingEventTimeline([]interface{}{event})
	var got []string
	for _, entry := range timeline.entries {
		got = append(got, entry.subject)
	}
	want := []string{"Alice[1]", "Alice[2]", "Alice[1]", "Bob", "u4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subjects = %#v, want %#v", got, want)
	}
}

func TestBuildMeetingEventTimeline_DisambiguatesNonTranscriptActors(t *testing.T) {
	events := []interface{}{
		map[string]interface{}{
			"event_type": "participant_joined",
			"payload": map[string]interface{}{
				"participant_joined_items": []interface{}{
					map[string]interface{}{"participant": map[string]interface{}{"id": "u1", "user_name": "Alex"}},
				},
			},
		},
		map[string]interface{}{
			"event_type": "chat_received",
			"payload": map[string]interface{}{
				"chat_received_items": []interface{}{
					map[string]interface{}{"operator": map[string]interface{}{"id": "u2", "user_name": "Alex"}, "content": "hello", "message_type": 1},
				},
			},
		},
		map[string]interface{}{
			"event_type": "magic_share_started",
			"payload": map[string]interface{}{
				"magic_share_started_items": []interface{}{
					map[string]interface{}{"operator": map[string]interface{}{"id": "u1", "user_name": "Alex"}},
				},
			},
		},
		map[string]interface{}{
			"event_type": "document_context_changed",
			"payload": map[string]interface{}{
				"document_context_changed_items": []interface{}{
					map[string]interface{}{"operator": map[string]interface{}{"id": "u2", "user_name": "Alex"}, "comment_focus": map[string]interface{}{"comment_id": "c1", "focused": true}},
				},
			},
		},
	}

	timeline := buildMeetingEventTimeline(events)
	var got []string
	for _, entry := range timeline.entries {
		got = append(got, entry.subject)
	}
	want := []string{"Alex[1]", "Alex[2]", "Alex[1]", "Alex[2]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subjects = %#v, want %#v", got, want)
	}
}
