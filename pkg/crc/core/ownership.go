package core

import (
	"errors"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

var (
	ErrNotAttributeOwner     = errors.New("ownership: federate is not the owner of this attribute")
	ErrAttributeAlreadyOwned = errors.New("ownership: attribute is already owned by another federate")
	ErrAttributeUnowned      = errors.New("ownership: attribute has no active owner")
)

// ObjectAttrKey uniquely identifies an attribute instance belonging to an object
type ObjectAttrKey struct {
	ObjectHandle common.ObjectInstanceHandle
	AttrHandle   common.AttributeHandle
}

// OwnershipManager coordinates IEEE 1516-2025 attribute ownership lifecycle:
// initial assignment upon registration, unconditional divestiture, acquisition, and ownership queries.
type OwnershipManager struct {
	mu       sync.RWMutex
	owners   map[ObjectAttrKey]common.FederateHandle
	divested map[ObjectAttrKey]bool
}

// NewOwnershipManager instantiates a thread-safe attribute ownership manager
func NewOwnershipManager() *OwnershipManager {
	return &OwnershipManager{
		owners:   make(map[ObjectAttrKey]common.FederateHandle),
		divested: make(map[ObjectAttrKey]bool),
	}
}

// RegisterInitialOwnership assigns initial ownership of attributes to the registering federate
func (m *OwnershipManager) RegisterInitialOwnership(objHandle common.ObjectInstanceHandle, attrs []common.AttributeHandle, owner common.FederateHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, attr := range attrs {
		key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
		m.owners[key] = owner
		delete(m.divested, key)
	}
}

// UnconditionalDivestiture immediately relinquishes ownership of the specified attributes
func (m *OwnershipManager) UnconditionalDivestiture(objHandle common.ObjectInstanceHandle, attrs []common.AttributeHandle, fedID common.FederateHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, attr := range attrs {
		key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
		currentOwner, exists := m.owners[key]
		if !exists || currentOwner != fedID {
			return ErrNotAttributeOwner
		}
	}

	for _, attr := range attrs {
		key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
		delete(m.owners, key)
		m.divested[key] = true
	}
	return nil
}

// AcquireAttributeOwnership transfers ownership of an unowned/divested attribute to the acquiring federate
func (m *OwnershipManager) AcquireAttributeOwnership(objHandle common.ObjectInstanceHandle, attrs []common.AttributeHandle, fedID common.FederateHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, attr := range attrs {
		key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
		if owner, exists := m.owners[key]; exists && owner != 0 && owner != fedID {
			return ErrAttributeAlreadyOwned
		}
	}

	for _, attr := range attrs {
		key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
		m.owners[key] = fedID
		delete(m.divested, key)
	}
	return nil
}

// QueryAttributeOwnership returns the current owner of an attribute instance (0 if unowned)
func (m *OwnershipManager) QueryAttributeOwnership(objHandle common.ObjectInstanceHandle, attr common.AttributeHandle) (common.FederateHandle, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
	owner, exists := m.owners[key]
	if !exists || owner == 0 {
		return 0, false
	}
	return owner, true
}

// DeleteObjectOwnership purges all ownership tracking for a deleted object instance
func (m *OwnershipManager) DeleteObjectOwnership(objHandle common.ObjectInstanceHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key := range m.owners {
		if key.ObjectHandle == objHandle {
			delete(m.owners, key)
		}
	}
	for key := range m.divested {
		if key.ObjectHandle == objHandle {
			delete(m.divested, key)
		}
	}
}

// RemoveFederate releases or divests all attributes owned by a resigning federate
func (m *OwnershipManager) RemoveFederate(fedID common.FederateHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, owner := range m.owners {
		if owner == fedID {
			delete(m.owners, key)
			m.divested[key] = true
		}
	}
}
