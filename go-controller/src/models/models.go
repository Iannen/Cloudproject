package models

import "encoding/json"

// Assignment represents the JSON structure of an assignment definition
type Assignment struct {
	NodeID string          `json:"node_id"`
	ID     string          `json:"id"`
	Role   string          `json:"role"`
	Config json.RawMessage `json:"config"`
}
