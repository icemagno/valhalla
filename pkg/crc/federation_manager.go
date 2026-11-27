package crc

import (
	"sync"
)

// FederationManager manages the lifecycle of all active federation executions in the CRC
type FederationManager struct {
	mu          sync.RWMutex
	federations map[string]*FederationExecution
	nextHandle  int32
}

// NewFederationManager instantiates the central federation manager
func NewFederationManager() *FederationManager {
	return &FederationManager{
		federations: make(map[string]*FederationExecution),
		nextHandle:  1,
	}
}

// CreateFederation creates a new federation execution with a unique name
func (m *FederationManager) CreateFederation(name string) (*FederationExecution, error) {
	return m.CreateFederationWithFOMs(name, nil)
}

// CreateFederationWithFOMs creates a new federation execution and ingests specified FOM modules
func (m *FederationManager) CreateFederationWithFOMs(name string, foms []string) (*FederationExecution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.federations[name]; exists {
		return nil, ErrFederationExecutionAlreadyExists
	}

	h := m.nextHandle
	m.nextHandle++

	fed := NewFederationExecution(h, name)
	m.federations[name] = fed

	// Ingest specified FOM modules
	for _, f := range foms {
		if err := IngestFOMIntoFederation(fed, f); err != nil {
			delete(m.federations, name)
			return nil, err
		}
	}

	// Ingest canonical defaults for standard federations
	IngestCanonicalDefaults(fed)

	return fed, nil
}

// GetOrCreateFederation retrieves an existing federation or creates one if it does not exist
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

// GetFederation looks up a federation execution by name
func (m *FederationManager) GetFederation(name string) (*FederationExecution, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fed, exists := m.federations[name]
	return fed, exists
}

// GetFirstFederation retrieves the first active federation execution
func (m *FederationManager) GetFirstFederation() (*FederationExecution, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, fed := range m.federations {
		return fed, true
	}
	return nil, false
}

// GetAllFederations returns a snapshot slice of all active federation executions
func (m *FederationManager) GetAllFederations() []*FederationExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feds := make([]*FederationExecution, 0, len(m.federations))
	for _, fed := range m.federations {
		feds = append(feds, fed)
	}
	return feds
}

// ListFederations returns a list of names of all active federations
func (m *FederationManager) ListFederations() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.federations))
	for name := range m.federations {
		names = append(names, name)
	}
	return names
}

// DestroyFederation removes the federation execution if no active federates remain
func (m *FederationManager) DestroyFederation(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fed, exists := m.federations[name]
	if !exists {
		return ErrFederationExecutionDoesNotExist
	}

	if len(fed.GetAllFederates()) > 0 {
		return ErrFederatesCurrentlyJoined // Federation still has active federate members
	}

	delete(m.federations, name)
	return nil
}
