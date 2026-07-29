package sdk

import "encoding/json"

type Module interface {
	Info() ModuleInfo
	Scout(target string, args []string) json.RawMessage
	Render(raw json.RawMessage) string
}

type ModuleNamespace struct {
	Name   string `json:"name"`
	Author string `json:"author"`
	Source string `json:"source"`
}

type ModuleInfo struct {
	Name         string            `json:"name"`
	Author       string            `json:"author"`
	Description  string            `json:"description"`
	Dependencies []ModuleNamespace `json:"dependencies"`
}
