package sdk

type Module interface {
    Info() ModuleInfo
    Scout(target string) string
}

type ModuleNamespace struct {
    Name string `json:"name"`
    Author string `json:"author"`
}

type ModuleInfo struct {
    Namespace ModuleNamespace `json:"namespace"`
    Description string `json:"description"`
    Dependencies []ModuleNamespace `json:"dependencies"`
}
