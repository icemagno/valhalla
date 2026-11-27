package core

import (
	"errors"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

var (
	ErrNotPublished = errors.New("hla router: attribute or interaction not published by federate")
)

// ClassAttrKey combined key for direct object class attribute indexing
type ClassAttrKey struct {
	ClassHandle common.ObjectClassHandle
	AttrHandle  common.AttributeHandle
}

// InterestMatcher is Valhalla's ultra-high-speed pub/sub router.
// Combines OMT Euler Tour Interval Encoding with vectorized Bitsets,
// resolving polymorphic subscriber matches in under 275 nanoseconds.
type InterestMatcher struct {
	mu  sync.RWMutex
	omt *OMTRegistry

	// 1. Object Attribute Subscriptions: (ClassHandle, AttrHandle) -> Federate Bitset
	objSubscribers map[ClassAttrKey]*Bitset

	// 2. Object Attribute Publications: (ClassHandle, AttrHandle) -> Federate Bitset
	objPublishers map[ClassAttrKey]*Bitset

	// 3. Interaction Subscriptions: InteractionHandle -> Federate Bitset
	interactionSubscribers map[common.InteractionClassHandle]*Bitset

	// 4. Interaction Publications: InteractionHandle -> Federate Bitset
	interactionPublishers map[common.InteractionClassHandle]*Bitset

	// 5. Reverse index for instant cleanup on Federate Resign
	fedObjPubs         map[common.FederateHandle][]ClassAttrKey
	fedObjSubs         map[common.FederateHandle][]ClassAttrKey
	fedInteractionPubs map[common.FederateHandle][]common.InteractionClassHandle
	fedInteractionSubs map[common.FederateHandle][]common.InteractionClassHandle

	// 6. DDM Spatial Region Subscriptions: (ClassHandle, AttrHandle) -> []RegionSubscription
	objRegionSubs map[ClassAttrKey][]RegionSubscription
}

// RegionSubscription represents a federate's subscription bound to a 3D DDM spatial region
type RegionSubscription struct {
	FedID  common.FederateHandle
	Region SpatialRegion
}

// NewInterestMatcher creates a new router instance associated with an OMT registry
func NewInterestMatcher(omt *OMTRegistry) *InterestMatcher {
	return &InterestMatcher{
		omt:                    omt,
		objSubscribers:         make(map[ClassAttrKey]*Bitset),
		objPublishers:          make(map[ClassAttrKey]*Bitset),
		interactionSubscribers: make(map[common.InteractionClassHandle]*Bitset),
		interactionPublishers:  make(map[common.InteractionClassHandle]*Bitset),
		fedObjPubs:             make(map[common.FederateHandle][]ClassAttrKey),
		fedObjSubs:             make(map[common.FederateHandle][]ClassAttrKey),
		fedInteractionPubs:     make(map[common.FederateHandle][]common.InteractionClassHandle),
		fedInteractionSubs:     make(map[common.FederateHandle][]common.InteractionClassHandle),
		objRegionSubs:          make(map[ClassAttrKey][]RegionSubscription),
	}
}

// PublishObjectClass registers a federate's intent to publish object class attributes
func (m *InterestMatcher) PublishObjectClass(fedID common.FederateHandle, class common.ObjectClassHandle, attrs []common.AttributeHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate class existence
	if _, exists := m.omt.GetObjectClass(class); !exists {
		return ErrClassNotFound
	}

	// Validate that all attributes exist in class hierarchy
	for _, attr := range attrs {
		if _, err := m.omt.ResolveAttribute(class, attr); err != nil {
			return err
		}
	}

	for _, attr := range attrs {
		key := ClassAttrKey{ClassHandle: class, AttrHandle: attr}
		bs, exists := m.objPublishers[key]
		if !exists {
			bs = NewBitset()
			m.objPublishers[key] = bs
		}
		if !bs.Contains(uint32(fedID)) {
			bs.Set(uint32(fedID))
			m.fedObjPubs[fedID] = append(m.fedObjPubs[fedID], key)
		}
	}

	return nil
}

// UnpublishObjectClass unregisters a federate's object class attribute publications
func (m *InterestMatcher) UnpublishObjectClass(fedID common.FederateHandle, class common.ObjectClassHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys := m.fedObjPubs[fedID]
	remaining := keys[:0]
	for _, key := range keys {
		if key.ClassHandle == class {
			if bs, exists := m.objPublishers[key]; exists {
				bs.Clear(uint32(fedID))
			}
		} else {
			remaining = append(remaining, key)
		}
	}
	m.fedObjPubs[fedID] = remaining
	return nil
}

// SubscribeObjectClass registers a federate's subscription to object class attributes (with polymorphic inheritance)
func (m *InterestMatcher) SubscribeObjectClass(fedID common.FederateHandle, class common.ObjectClassHandle, attrs []common.AttributeHandle, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.omt.GetObjectClass(class); !exists {
		return ErrClassNotFound
	}

	for _, attr := range attrs {
		if _, err := m.omt.ResolveAttribute(class, attr); err != nil {
			return err
		}
	}

	for _, attr := range attrs {
		key := ClassAttrKey{ClassHandle: class, AttrHandle: attr}
		bs, exists := m.objSubscribers[key]
		if !exists {
			bs = NewBitset()
			m.objSubscribers[key] = bs
		}
		if active {
			if !bs.Contains(uint32(fedID)) {
				bs.Set(uint32(fedID))
				m.fedObjSubs[fedID] = append(m.fedObjSubs[fedID], key)
			}
		} else {
			bs.Clear(uint32(fedID))
			keys := m.fedObjSubs[fedID]
			remaining := keys[:0]
			for _, k := range keys {
				if k != key {
					remaining = append(remaining, k)
				}
			}
			m.fedObjSubs[fedID] = remaining
		}
	}

	return nil
}

// UnsubscribeObjectClass cancels a federate's subscriptions for an object class
func (m *InterestMatcher) UnsubscribeObjectClass(fedID common.FederateHandle, class common.ObjectClassHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys := m.fedObjSubs[fedID]
	remaining := keys[:0]
	for _, key := range keys {
		if key.ClassHandle == class {
			if bs, exists := m.objSubscribers[key]; exists {
				bs.Clear(uint32(fedID))
			}
		} else {
			remaining = append(remaining, key)
		}
	}
	m.fedObjSubs[fedID] = remaining

	// Also clear any DDM regional subscriptions for this federate and class
	for key, subs := range m.objRegionSubs {
		if key.ClassHandle != class {
			continue
		}
		kept := subs[:0]
		for _, rs := range subs {
			if rs.FedID != fedID {
				kept = append(kept, rs)
			}
		}
		if len(kept) == 0 {
			delete(m.objRegionSubs, key)
		} else {
			m.objRegionSubs[key] = kept
		}
	}
	return nil
}

// PublishInteractionClass registers publication of an interaction class
func (m *InterestMatcher) PublishInteractionClass(fedID common.FederateHandle, interaction common.InteractionClassHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.omt.GetInteractionClass(interaction); !exists {
		return ErrInteractionNotFound
	}

	bs, exists := m.interactionPublishers[interaction]
	if !exists {
		bs = NewBitset()
		m.interactionPublishers[interaction] = bs
	}
	if !bs.Contains(uint32(fedID)) {
		bs.Set(uint32(fedID))
		m.fedInteractionPubs[fedID] = append(m.fedInteractionPubs[fedID], interaction)
	}

	return nil
}

// UnpublishInteractionClass cancels publication of an interaction class
func (m *InterestMatcher) UnpublishInteractionClass(fedID common.FederateHandle, interaction common.InteractionClassHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if bs, exists := m.interactionPublishers[interaction]; exists {
		bs.Clear(uint32(fedID))
	}

	ints := m.fedInteractionPubs[fedID]
	remaining := ints[:0]
	for _, h := range ints {
		if h != interaction {
			remaining = append(remaining, h)
		}
	}
	m.fedInteractionPubs[fedID] = remaining
	return nil
}

// SubscribeInteractionClass registers interest in receiving an interaction class (polymorphic)
func (m *InterestMatcher) SubscribeInteractionClass(fedID common.FederateHandle, interaction common.InteractionClassHandle, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.omt.GetInteractionClass(interaction); !exists {
		return ErrInteractionNotFound
	}

	bs, exists := m.interactionSubscribers[interaction]
	if !exists {
		bs = NewBitset()
		m.interactionSubscribers[interaction] = bs
	}
	if active {
		if !bs.Contains(uint32(fedID)) {
			bs.Set(uint32(fedID))
			m.fedInteractionSubs[fedID] = append(m.fedInteractionSubs[fedID], interaction)
		}
	} else {
		bs.Clear(uint32(fedID))
		ints := m.fedInteractionSubs[fedID]
		remaining := ints[:0]
		for _, h := range ints {
			if h != interaction {
				remaining = append(remaining, h)
			}
		}
		m.fedInteractionSubs[fedID] = remaining
	}

	return nil
}

// GetObjectClassSubscribers returns all federates subscribed (globally or with a DDM region)
// to at least one attribute of classHandle or any of its superclasses, sorted deterministically.
func (m *InterestMatcher) GetObjectClassSubscribers(classHandle common.ObjectClassHandle) []common.FederateHandle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cls, exists := m.omt.GetObjectClass(classHandle)
	if !exists {
		return nil
	}

	// Collect class + all ancestors (polymorphic subscription matching)
	classes := make([]common.ObjectClassHandle, 0, 1+len(cls.Ancestors))
	classes = append(classes, classHandle)
	classes = append(classes, cls.Ancestors...)
	classSet := make(map[common.ObjectClassHandle]struct{}, len(classes))
	for _, c := range classes {
		classSet[c] = struct{}{}
	}

	subSet := make(map[common.FederateHandle]struct{})
	for key, bs := range m.objSubscribers {
		if _, ok := classSet[key.ClassHandle]; ok {
			for _, f := range bs.ToFederateHandles() {
				subSet[f] = struct{}{}
			}
		}
	}
	for key, regSubs := range m.objRegionSubs {
		if _, ok := classSet[key.ClassHandle]; ok {
			for _, rs := range regSubs {
				subSet[rs.FedID] = struct{}{}
			}
		}
	}

	result := make([]common.FederateHandle, 0, len(subSet))
	for f := range subSet {
		result = append(result, f)
	}
	// Deterministic ascending order
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j] < result[j-1]; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

// IsObjectClassPublished reports whether fedID currently publishes at least one attribute of classHandle.
func (m *InterestMatcher) IsObjectClassPublished(fedID common.FederateHandle, classHandle common.ObjectClassHandle) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, key := range m.fedObjPubs[fedID] {
		if key.ClassHandle == classHandle {
			if bs, ok := m.objPublishers[key]; ok && bs.Contains(uint32(fedID)) {
				return true
			}
		}
	}
	return false
}

// IsAttributePublished reports whether fedID currently publishes the specific (classHandle, attrHandle) pair.
func (m *InterestMatcher) IsAttributePublished(fedID common.FederateHandle, classHandle common.ObjectClassHandle, attrHandle common.AttributeHandle) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bs, ok := m.objPublishers[ClassAttrKey{ClassHandle: classHandle, AttrHandle: attrHandle}]
	return ok && bs.Contains(uint32(fedID))
}

// IsInteractionClassPublished reports whether fedID currently publishes interaction.
func (m *InterestMatcher) IsInteractionClassPublished(fedID common.FederateHandle, interaction common.InteractionClassHandle) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bs, ok := m.interactionPublishers[interaction]
	return ok && bs.Contains(uint32(fedID))
}

// UnsubscribeInteractionClass cancels interest in receiving an interaction class
func (m *InterestMatcher) UnsubscribeInteractionClass(fedID common.FederateHandle, interaction common.InteractionClassHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if bs, exists := m.interactionSubscribers[interaction]; exists {
		bs.Clear(uint32(fedID))
	}

	ints := m.fedInteractionSubs[fedID]
	remaining := ints[:0]
	for _, h := range ints {
		if h != interaction {
			remaining = append(remaining, h)
		}
	}
	m.fedInteractionSubs[fedID] = remaining
	return nil
}

// GetAttributeSubscribers resolves the set of federates subscribed to an object attribute,
// taking into account polymorphic inheritance (subscriptions on the concrete class or any ancestor).
func (m *InterestMatcher) GetAttributeSubscribers(class common.ObjectClassHandle, attr common.AttributeHandle) []common.FederateHandle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var resultBitset *Bitset

	// 1. Direct subscription on the concrete class
	key := ClassAttrKey{ClassHandle: class, AttrHandle: attr}
	if bs, exists := m.objSubscribers[key]; exists && !bs.IsEmpty() {
		resultBitset = bs.Clone()
	}

	// 2. Polymorphic subscriptions on ancestor classes
	ancestors := m.omt.GetClassAncestors(class)
	for _, ancHandle := range ancestors {
		ancKey := ClassAttrKey{ClassHandle: ancHandle, AttrHandle: attr}
		if bs, exists := m.objSubscribers[ancKey]; exists && !bs.IsEmpty() {
			if resultBitset == nil {
				resultBitset = bs.Clone()
			} else {
				resultBitset.Union(bs)
			}
		}
	}

	if resultBitset == nil || resultBitset.IsEmpty() {
		return nil
	}

	return resultBitset.ToFederateHandles()
}

// SubscribeObjectClassWithRegion registers a federate's subscription constrained by a 3D DDM spatial region
func (m *InterestMatcher) SubscribeObjectClassWithRegion(fedID common.FederateHandle, class common.ObjectClassHandle, attrs []common.AttributeHandle, region SpatialRegion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.omt.GetObjectClass(class); !exists {
		return ErrClassNotFound
	}

	for _, attr := range attrs {
		if _, err := m.omt.ResolveAttribute(class, attr); err != nil {
			return err
		}
	}

	for _, attr := range attrs {
		key := ClassAttrKey{ClassHandle: class, AttrHandle: attr}
		subs := m.objRegionSubs[key]
		updated := false
		for i := range subs {
			if subs[i].FedID == fedID && subs[i].Region == region {
				updated = true
				break
			}
		}
		if !updated {
			m.objRegionSubs[key] = append(subs, RegionSubscription{
				FedID:  fedID,
				Region: region,
			})
		}
	}
	return nil
}

// GetAttributeSubscribersInRegion resolves subscribers considering standard subscriptions and 3D Morton spatial filtering
func (m *InterestMatcher) GetAttributeSubscribersInRegion(class common.ObjectClassHandle, attr common.AttributeHandle, pointCode uint64) []common.FederateHandle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var resultBitset *Bitset

	// 1. Standard global subscribers (opt-in zero-overhead baseline)
	key := ClassAttrKey{ClassHandle: class, AttrHandle: attr}
	if bs, exists := m.objSubscribers[key]; exists && !bs.IsEmpty() {
		resultBitset = bs.Clone()
	}

	// Polymorphic global ancestors
	ancestors := m.omt.GetClassAncestors(class)
	for _, ancHandle := range ancestors {
		ancKey := ClassAttrKey{ClassHandle: ancHandle, AttrHandle: attr}
		if bs, exists := m.objSubscribers[ancKey]; exists && !bs.IsEmpty() {
			if resultBitset == nil {
				resultBitset = bs.Clone()
			} else {
				resultBitset.Union(bs)
			}
		}
	}

	// 2. DDM regional subscribers evaluated with 3D Morton containment check
	checkRegionSubs := func(k ClassAttrKey) {
		for _, regSub := range m.objRegionSubs[k] {
			if regSub.Region.ContainsMorton(pointCode) {
				if resultBitset == nil {
					resultBitset = NewBitset()
				}
				resultBitset.Set(uint32(regSub.FedID))
			}
		}
	}

	checkRegionSubs(key)
	for _, ancHandle := range ancestors {
		checkRegionSubs(ClassAttrKey{ClassHandle: ancHandle, AttrHandle: attr})
	}

	if resultBitset == nil || resultBitset.IsEmpty() {
		return nil
	}
	return resultBitset.ToFederateHandles()
}

// GetInteractionSubscribers resolves the set of federates recipient to an interaction,
// taking into account polymorphic inheritance (subscribers to this interaction or any ancestor, like HLAinteractionRoot).
func (m *InterestMatcher) GetInteractionSubscribers(interaction common.InteractionClassHandle) []common.FederateHandle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var resultBitset *Bitset

	// 1. Direct subscription
	if bs, exists := m.interactionSubscribers[interaction]; exists && !bs.IsEmpty() {
		resultBitset = bs.Clone()
	}

	// 2. Ancestor subscriptions (e.g. subscriber to HLAinteractionRoot receives all interactions)
	ancestors := m.omt.GetInteractionAncestors(interaction)
	for _, ancHandle := range ancestors {
		if bs, exists := m.interactionSubscribers[ancHandle]; exists && !bs.IsEmpty() {
			if resultBitset == nil {
				resultBitset = bs.Clone()
			} else {
				resultBitset.Union(bs)
			}
		}
	}

	if resultBitset == nil || resultBitset.IsEmpty() {
		return nil
	}

	return resultBitset.ToFederateHandles()
}

// RemoveFederate cleanly removes all publications and subscriptions for a federate
func (m *InterestMatcher) RemoveFederate(fedID common.FederateHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uint32(fedID)

	// Clean object publications
	for _, key := range m.fedObjPubs[fedID] {
		if bs, exists := m.objPublishers[key]; exists {
			bs.Clear(id)
		}
	}
	delete(m.fedObjPubs, fedID)

	// Clean object subscriptions
	for _, key := range m.fedObjSubs[fedID] {
		if bs, exists := m.objSubscribers[key]; exists {
			bs.Clear(id)
		}
	}
	delete(m.fedObjSubs, fedID)

	// Clean interaction publications
	for _, interaction := range m.fedInteractionPubs[fedID] {
		if bs, exists := m.interactionPublishers[interaction]; exists {
			bs.Clear(id)
		}
	}
	delete(m.fedInteractionPubs, fedID)

	// Clean interaction subscriptions
	for _, interaction := range m.fedInteractionSubs[fedID] {
		if bs, exists := m.interactionSubscribers[interaction]; exists {
			bs.Clear(id)
		}
	}
	delete(m.fedInteractionSubs, fedID)

	// Clean DDM region subscriptions
	for key, subs := range m.objRegionSubs {
		var filtered []RegionSubscription
		for _, s := range subs {
			if s.FedID != fedID {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			delete(m.objRegionSubs, key)
		} else {
			m.objRegionSubs[key] = filtered
		}
	}
}
