package sdk

import "encoding/json"

// Module is what a module binary implements and hands to [Serve].
//
// Scout does the work and returns its result as JSON. It reports failure by
// returning an error, which fails that one call and leaves the module running
// a module must not exit on a bad target.
// Render turns a result into text for a human.
type Module interface {
	Info() ModuleInfo
	Scout(target string, args []string) (json.RawMessage, error)
	Render(raw json.RawMessage) string
}

type ModuleNamespace struct {
	Name   string `json:"name"`
	Author string `json:"author"`
}

type ModuleInfo struct {
	Name         string            `json:"name"`
	Author       string            `json:"author"`
	Description  string            `json:"description"`
	Dependencies []ModuleNamespace `json:"dependencies"`
}

func (m *ModuleInfo) IsValid() bool {
	return m.Name != "" && m.Author != "" && m.Description != ""
}
