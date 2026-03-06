package adapterprotocol

import "specdown/internal/specdown/core"

const Version = "specdown-adapter/v1"

type Request struct {
	Type  string `json:"type"`
	Cases []Case `json:"cases,omitempty"`
}

type Case struct {
	Kind   string      `json:"kind"`
	Info   string      `json:"info"`
	Source string      `json:"source"`
	ID     core.SpecID `json:"id"`
}

type Response struct {
	Type       string       `json:"type"`
	Blocks     []string     `json:"blocks,omitempty"`
	Fixtures   []string     `json:"fixtures,omitempty"`
	ID         *core.SpecID `json:"id,omitempty"`
	Label      string       `json:"label,omitempty"`
	Message    string       `json:"message,omitempty"`
	Details    string       `json:"details,omitempty"`
	DurationMs int64        `json:"durationMs,omitempty"`
}
