package models

import "encoding/json"

type Assignment struct {
	NodeID string          `json:"node_id"`
	ID     string          `json:"id"`
	Role   string          `json:"role"`
	Config json.RawMessage `json:"config"`
}
