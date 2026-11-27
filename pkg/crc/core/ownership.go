package core

import (
	"errors"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

var (
	ErrNotAttributeOwner     = errors.New("ownership: federate is not the owner of this attribute")
	ErrAttributeAlreadyOwned = errors.New("ownership: attribute is already owned by another federate")
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
	objAttrs map[common.ObjectInstanceHandle]map[common.AttributeHandle]struct{}
	fedOwned map[common.FederateHandle]map[ObjectAttrKey]struct{}
}

// NewOwnershipManager instantiates a thread-safe attribute ownership manager
func NewOwnershipManager() *OwnershipManager {
	return &OwnershipManager{
		owners:   make(map[ObjectAttrKey]common.FederateHandle),
		divested: make(map[ObjectAttrKey]bool),
		objAttrs: make(map[common.ObjectInstanceHandle]map[common.AttributeHandle]struct{}),
		fedOwned: make(map[common.FederateHandle]map[ObjectAttrKey]struct{}),
	}
}

func (m *OwnershipManager) trackObjAttrLocked(key ObjectAttrKey) {
	attrs, ok := m.objAttrs[key.ObjectHandle]
	if !ok {
		attrs = make(map[common.AttributeHandle]struct{})
		m.objAttrs[key.ObjectHandle] = attrs
	}
	attrs[key.AttrHandle] = struct{}{}
}

func (m *OwnershipManager) assignOwnerLocked(key ObjectAttrKey, owner common.FederateHandle) {
	if prev, ok := m.owners[key]; ok && prev != owner {
		if set, ok2 := m.fedOwned[prev]; ok2 {
			delete(set, key)
			if len(set) == 0 {
				delete(m.fedOwned, prev)
			}
		}
	}
	m.owners[key] = owner
	delete(m.divested, key)
	m.trackObjAttrLocked(key)
	if owner != 0 {
		set, ok := m.fedOwned[owner]
		if !ok {
			set = make(map[ObjectAttrKey]struct{})
			m.fedOwned[owner] = set
		}
		set[key] = struct{}{}
	}
}

func (m *OwnershipManager) unassignOwnerLocked(key ObjectAttrKey, owner common.FederateHandle) {
	delete(m.owners, key)
	if set, ok := m.fedOwned[owner]; ok {
		delete(set, key)
		if len(set) == 0 {
			delete(m.fedOwned, owner)
		}
	}
}

// RegisterInitialOwnership assigns initial ownership of attributes to the registering federate
func (m *OwnershipManager) RegisterInitialOwnership(objHandle common.ObjectInstanceHandle, attrs []common.AttributeHandle, owner common.FederateHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, attr := range attrs {
		key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
		m.assignOwnerLocked(key, owner)
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
		m.unassignOwnerLocked(key, fedID)
		m.divested[key] = true
		m.trackObjAttrLocked(key)
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
		m.assignOwnerLocked(key, fedID)
	}
	return nil
}

// IsAttributeDivested returns true if the attribute was explicitly divested and currently has no owner
func (m *OwnershipManager) IsAttributeDivested(objHandle common.ObjectInstanceHandle, attr common.AttributeHandle) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
	return m.divested[key]
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

// DeleteObjectOwnership purges all ownership tracking for a deleted object instance in O(k)
func (m *OwnershipManager) DeleteObjectOwnership(objHandle common.ObjectInstanceHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	attrs, ok := m.objAttrs[objHandle]
	if !ok {
		return
	}
	for attr := range attrs {
		key := ObjectAttrKey{ObjectHandle: objHandle, AttrHandle: attr}
		if owner, hasOwner := m.owners[key]; hasOwner {
			m.unassignOwnerLocked(key, owner)
		}
		delete(m.divested, key)
	}
	delete(m.objAttrs, objHandle)
}

// RemoveFederate releases or divests all attributes owned by a resigning federate in O(k)
func (m *OwnershipManager) RemoveFederate(fedID common.FederateHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ownedSet, ok := m.fedOwned[fedID]
	if !ok {
		return
	}
	for key := range ownedSet {
		delete(m.owners, key)
		m.divested[key] = true
	}
	delete(m.fedOwned, fedID)
}
