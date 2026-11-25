package crc

import (
	"sync"
)

// FederationManager gerencia o ciclo de vida de todas as execuções de federação ativas no CRC
type FederationManager struct {
	mu          sync.RWMutex
	federations map[string]*FederationExecution
	nextHandle  int32
}

// NewFederationManager instancia o gerenciador de federações central
func NewFederationManager() *FederationManager {
	return &FederationManager{
		federations: make(map[string]*FederationExecution),
		nextHandle:  1,
	}
}

// CreateFederation cria uma nova federação com nome exclusivo
func (m *FederationManager) CreateFederation(name string) (*FederationExecution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.federations[name]; exists {
		return nil, ErrFederationExecutionAlreadyExists
	}

	h := m.nextHandle
	m.nextHandle++

	fed := NewFederationExecution(h, name)
	m.federations[name] = fed
	return fed, nil
}

// GetOrCreateFederation obtém uma federação existente ou cria se não existir
func (m *FederationManager) GetOrCreateFederation(name string) *FederationExecution {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fed, exists := m.federations[name]; exists {
		return fed
	}

	h := m.nextHandle
	m.nextHandle++

	fed := NewFederationExecution(h, name)
	m.federations[name] = fed
	return fed
}

// GetFederation busca uma federação pelo nome
func (m *FederationManager) GetFederation(name string) (*FederationExecution, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fed, exists := m.federations[name]
	return fed, exists
}

// DestroyFederation remove a federação se não houver federados ativos
func (m *FederationManager) DestroyFederation(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fed, exists := m.federations[name]
	if !exists {
		return ErrFederationExecutionDoesNotExist
	}

	if len(fed.GetAllFederates()) > 0 {
		return ErrFederatesCurrentlyJoined // Federação ainda possui participantes ativos
	}

	delete(m.federations, name)
	return nil
}
