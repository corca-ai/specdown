package adapterprotocol

const Version = "specdown-adapter/v1"

type SpecID struct {
	File        string   `json:"file"`
	HeadingPath []string `json:"headingPath"`
	Ordinal     int      `json:"ordinal"`
}

type Binding struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Request struct {
	Type     string `json:"type"`
	Protocol string `json:"protocol,omitempty"`
	Case     *Case  `json:"case,omitempty"`
}

type Case struct {
	ID           SpecID    `json:"id"`
	Kind         string    `json:"kind"`
	Block        string    `json:"block"`
	Source       string    `json:"source"`
	Fixture      string    `json:"fixture,omitempty"`
	Columns      []string  `json:"columns,omitempty"`
	Cells        []string  `json:"cells,omitempty"`
	CaptureNames []string  `json:"captureNames,omitempty"`
	Bindings     []Binding `json:"bindings,omitempty"`
}

type Response struct {
	Type       string    `json:"type"`
	Blocks     []string  `json:"blocks,omitempty"`
	Fixtures   []string  `json:"fixtures,omitempty"`
	ID         *SpecID   `json:"id,omitempty"`
	Label      string    `json:"label,omitempty"`
	Message    string    `json:"message,omitempty"`
	Details    string    `json:"details,omitempty"`
	DurationMs int64     `json:"durationMs,omitempty"`
	Bindings   []Binding `json:"bindings,omitempty"`
}
