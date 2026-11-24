package metamodel

import "sync"

var (
	registryMu sync.RWMutex
	classes    = make(map[string]*ClassDesc)
)

// RegisterClassDesc registra um descritor de classe no catálogo global
func RegisterClassDesc(cls *ClassDesc) {
	if cls == nil || cls.Name == "" {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	classes[cls.Name] = cls
}

// LookupClassDesc busca um descritor de classe pelo nome totalmente qualificado
func LookupClassDesc(name string) (*ClassDesc, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	cls, found := classes[name]
	return cls, found
}
