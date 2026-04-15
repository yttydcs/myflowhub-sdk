package transport

// 本文件覆盖 SDK 客户端侧中与 `message` 相关的行为。

import (
	"encoding/json"
	"testing"
)

func TestEncodeMessage_ActionRequired(t *testing.T) {
	if _, err := EncodeMessage("", map[string]any{"k": "v"}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := EncodeMessage("   ", map[string]any{"k": "v"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDecodeMessage_Validation(t *testing.T) {
	if _, err := DecodeMessage(nil); err == nil {
		t.Fatalf("expected error for empty payload")
	}
	if _, err := DecodeMessage([]byte("not-json")); err == nil {
		t.Fatalf("expected error for invalid json")
	}
	if _, err := DecodeMessage([]byte(`{"data":{"k":1}}`)); err == nil {
		t.Fatalf("expected error for missing action")
	}
}

func TestEncodeDecodeMessage_Roundtrip(t *testing.T) {
	payload, err := EncodeMessage("node_echo", map[string]any{"message": "ping"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	m, err := DecodeMessage(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.Action != "node_echo" {
		t.Fatalf("unexpected action: %q", m.Action)
	}
	var data map[string]any
	if err := json.Unmarshal(m.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data["message"] != "ping" {
		t.Fatalf("unexpected data: %+v", data)
	}
}
