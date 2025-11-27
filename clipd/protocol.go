package clipd

type RequestType int

const (
	RequestTypeClipboard RequestType = iota
	RequestTypeRun
	RequestTypePipe
)

type Request struct {
	Type       RequestType `json:"type"`
	Data       string      `json:"data,omitempty"`
	Args       []string    `json:"args,omitempty"`
	WorkingDir string      `json:"workingDir,omitempty"`
	Password   string      `json:"password,omitempty"`
	Stdin      string      `json:"stdin,omitempty"`
}
