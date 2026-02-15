package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

var (
	ErrActionRequired = errors.New("action is required")
	ErrPayloadEmpty   = errors.New("payload is empty")
)

func EncodeMessage(action string, data any) ([]byte, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return nil, ErrActionRequired
	}

	// 注意：这里用 any 承载 data，避免调用方必须手工构造 RawMessage。
	// DecodeMessage 则使用 RawMessage 承载 data，避免二次反序列化。
	wire := struct {
		Action string `json:"action"`
		Data   any    `json:"data,omitempty"`
	}{
		Action: action,
		Data:   data,
	}
	return json.Marshal(wire)
}

func DecodeMessage(payload []byte) (Message, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return Message{}, ErrPayloadEmpty
	}
	var m Message
	if err := json.Unmarshal(payload, &m); err != nil {
		return Message{}, err
	}
	m.Action = strings.TrimSpace(m.Action)
	if m.Action == "" {
		return Message{}, ErrActionRequired
	}
	return m, nil
}
