package api

// Client → Server messages

type InputMessage struct {
	Type string `json:"type"` // "input"
	Data string `json:"data"`
}

type ResizeMessage struct {
	Type string `json:"type"` // "resize"
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

type PingMessage struct {
	Type string `json:"type"` // "ping"
}

// Server → Client messages

type OutputMessage struct {
	Type string `json:"type"` // "output"
	Data string `json:"data"`
}

type ErrorMessage struct {
	Type    string `json:"type"` // "error"
	Message string `json:"message"`
}

type PongMessage struct {
	Type string `json:"type"` // "pong"
}
