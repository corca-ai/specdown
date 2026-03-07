package adapterprotocol

type Binding struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Request struct {
	Type string `json:"type"`
	ID   int    `json:"id,omitempty"`
	Case *Case  `json:"case,omitempty"`
}

type Case struct {
	Kind          string            `json:"kind"`
	Block         string            `json:"block"`
	Source        string            `json:"source"`
	Fixture       string            `json:"fixture,omitempty"`
	FixtureParams map[string]string `json:"fixtureParams,omitempty"`
	Columns       []string          `json:"columns,omitempty"`
	Cells         []string          `json:"cells,omitempty"`
	CaptureNames  []string          `json:"captureNames,omitempty"`
	Bindings      []Binding         `json:"bindings,omitempty"`
}

type Response struct {
	ID       int       `json:"id"`
	Type     string    `json:"type"`
	Message  string    `json:"message,omitempty"`
	Bindings []Binding `json:"bindings,omitempty"`
}
