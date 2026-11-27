package crc

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// ProtocolType identifies the transport and serialization dialect used by the federate
type ProtocolType = common.ProtocolType

const (
	ProtocolUnknown = common.ProtocolUnknown
	ProtocolNative  = common.ProtocolNative
	ProtocolPortico = common.ProtocolPortico
	ProtocolWeb     = common.ProtocolWeb
)

// Session defines the transport interface for asynchronous message dispatch to a federate
type Session interface {
	FederateID() int32
	Protocol() ProtocolType
	SendNotification(msgType uint8, payload []byte) error
	SendNotificationWithMerkle(msgType uint8, payload []byte, merkleRoot [32]byte) error
	Close() error
}

// DomainEventSink allows protocol sessions (Native SSZ, Portico JOS, Web JSON) to serialize
// typed HLA domain events directly to their native wire format in a single pass.
type DomainEventSink interface {
	SendDiscoverObject(objHandle, classHandle int32, objName string) error
	SendRemoveObject(objHandle int32, tag []byte) error
	SendReflectAttributes(objHandle int32, attrs map[common.AttributeHandle][]byte, tag []byte, hasTime bool, timestamp float64, merkleRoot [32]byte) error
	SendReceiveInteraction(classHandle int32, params map[common.ParameterHandle][]byte, tag []byte, hasTime bool, timestamp float64) error
	SendTimeAdvanceGrant(grantedTime float64) error
	SendAnnounceSyncPoint(label string, tag []byte) error
	SendFederationSynchronized(label string) error
}

// Federate represents a participant node in a federation execution on the CRC
// Note: Logical time state is owned exclusively by core.TimeManager (single source of truth).
type Federate struct {
	ID           int32
	Name         string
	FederateType string
	Protocol     ProtocolType
	Session      Session
	JoinedAt     time.Time
}

// SyncPoint represents an active synchronization point in the federation
type SyncPoint struct {
	Label             string
	Tag               []byte
	RegistrarID       int32
	AchievedFederates map[int32]bool
	IsSynchronized    bool
	CreatedAt         time.Time
}

// NewSyncPoint instantiates a new synchronization point
func NewSyncPoint(label string, tag []byte, registrarID int32) *SyncPoint {
	return &SyncPoint{
		Label:             label,
		Tag:               tag,
		RegistrarID:       registrarID,
		AchievedFederates: make(map[int32]bool),
		CreatedAt:         time.Now(),
	}
}

// ObjectInstance represents an active object instance registered in the simulation
type ObjectInstance struct {
	Handle      int32
	ClassHandle int32
	Name        string
	OwnerID     int32
	Attributes  map[int32][]byte
	MerkleRoot  [32]byte
	UpdatedAt   time.Time
}

// FederationExecution represents the runtime state of a federation execution on the CRC
type FederationExecution struct {
	mu sync.RWMutex

	Handle int32
	Name   string

	federates   map[int32]*Federate
	nameToID    map[string]int32
	syncPoints  map[string]*SyncPoint
	objects     map[int32]*ObjectInstance
	nameToObjID map[string]int32

	nextFederateHandle int32
	nextObjectHandle   int32
	nextRegionHandle   uint32
	regions            map[uint32]*core.SpatialRegion
	regionDims         map[uint32][]uint32
	regionOwner        map[uint32]int32

	omt          *core.OMTRegistry
	router       *core.InterestMatcher
	stateTree    *core.FederationStateTree
	timeMgr      *core.TimeManager
	ownershipMgr *core.OwnershipManager

	merkleAuditEnabled bool
}

// NewFederationExecution creates a new federation execution with initialized counters and maps
func NewFederationExecution(handle int32, name string) *FederationExecution {
	omt := core.NewOMTRegistry()
	router := core.NewInterestMatcher(omt)
	stateTree := core.NewFederationStateTree()
	timeMgr := core.NewTimeManager()
	ownershipMgr := core.NewOwnershipManager()

	return &FederationExecution{
		Handle:             handle,
		Name:               name,
		federates:          make(map[int32]*Federate),
		nameToID:           make(map[string]int32),
		syncPoints:         make(map[string]*SyncPoint),
		objects:            make(map[int32]*ObjectInstance),
		nameToObjID:        make(map[string]int32),
		nextFederateHandle: 1, // Handles start at 1
		nextObjectHandle:   1,
		nextRegionHandle:   1,
		regions:            make(map[uint32]*core.SpatialRegion),
		regionDims:         make(map[uint32][]uint32),
		regionOwner:        make(map[uint32]int32),
		omt:                omt,
		router:             router,
		stateTree:          stateTree,
		timeMgr:            timeMgr,
		ownershipMgr:       ownershipMgr,
		merkleAuditEnabled: true,
	}
}

// SetMerkleAuditEnabled configures whether cryptographic Merkle state tracking is active.
// When transitioning from disabled to enabled in an active federation, it resynchronizes
// all active objects into the state tree so that the global root immediately reflects 100% of state.
func (f *FederationExecution) SetMerkleAuditEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	wasEnabled := f.merkleAuditEnabled
	f.merkleAuditEnabled = enabled

	if !wasEnabled && enabled && f.stateTree != nil {
		for h, obj := range f.objects {
			if len(obj.Attributes) > 0 {
				attrMap := make(map[common.AttributeHandle][]byte, len(obj.Attributes))
				for k, v := range obj.Attributes {
					attrMap[common.AttributeHandle(k)] = v
				}
				obj.MerkleRoot = core.ComputeObjectMerkleRoot(attrMap)
			}
			f.stateTree.UpdateObjectState(common.ObjectInstanceHandle(h), obj.MerkleRoot)
		}
	}
}

// IsMerkleAuditEnabled returns whether cryptographic Merkle state tracking is active
func (f *FederationExecution) IsMerkleAuditEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.merkleAuditEnabled
}

// OMT returns the Object Model Template (OMT) registry of the federation
func (f *FederationExecution) OMT() *core.OMTRegistry {
	return f.omt
}

// Router returns the publish/subscribe router (InterestMatcher) of the federation
func (f *FederationExecution) Router() *core.InterestMatcher {
	return f.router
}

// StateTree returns the cryptographic Merkle state tree of the federation
func (f *FederationExecution) StateTree() *core.FederationStateTree {
	return f.stateTree
}

// TimeManager returns the deterministic time advance coordinator of the federation
func (f *FederationExecution) TimeManager() *core.TimeManager {
	return f.timeMgr
}

// OwnershipManager returns the attribute ownership manager of the federation
func (f *FederationExecution) OwnershipManager() *core.OwnershipManager {
	return f.ownershipMgr
}

// Join registers a federate in the federation and assigns its unique sequential handle
func (f *FederationExecution) Join(name, fedType string, proto ProtocolType, sess Session) (*Federate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nameToID[name]; exists {
		return nil, ErrFederateNameAlreadyInUse
	}

	id := f.nextFederateHandle
	f.nextFederateHandle++

	fed := &Federate{
		ID:           id,
		Name:         name,
		FederateType: fedType,
		Protocol:     proto,
		Session:      sess,
		JoinedAt:     time.Now(),
	}

	f.federates[id] = fed
	f.nameToID[name] = id

	if f.timeMgr != nil {
		f.timeMgr.RegisterFederate(common.FederateHandle(id))
	}

	return fed, nil
}

// Resign removes the federate from the federation, freeing its name and session
func (f *FederationExecution) Resign(id int32) (*Federate, error) {
	f.mu.Lock()

	fed, exists := f.federates[id]
	if !exists {
		f.mu.Unlock()
		return nil, ErrFederateNotFound
	}

	delete(f.federates, id)
	delete(f.nameToID, fed.Name)

	// Remove from pending synchronization points
	for _, sp := range f.syncPoints {
		delete(sp.AchievedFederates, id)
	}

	// Clear federate interests and publications in router
	if f.router != nil {
		f.router.RemoveFederate(common.FederateHandle(id))
	}

	// Remove federate from time manager and capture any unblocked grants
	var grants []core.TimeGrantEvent
	if f.timeMgr != nil {
		grants = f.timeMgr.UnregisterFederate(common.FederateHandle(id))
	}

	// Clear attribute ownership held by federate
	if f.ownershipMgr != nil {
		f.ownershipMgr.RemoveFederate(common.FederateHandle(id))
	}

	// Clean up DDM regions owned by the resigning federate to prevent memory leaks
	for regHandle, ownerID := range f.regionOwner {
		if ownerID == id {
			delete(f.regions, regHandle)
			delete(f.regionDims, regHandle)
			delete(f.regionOwner, regHandle)
		}
	}

	// Purge completed synchronization points when the federation becomes empty
	if len(f.federates) == 0 {
		for k := range f.syncPoints {
			delete(f.syncPoints, k)
		}
	}

	f.mu.Unlock()

	// Dispatch time advance grants outside f.mu to avoid lock inversion/deadlock
	if len(grants) > 0 {
		dispatchTimeGrants(f, grants)
	}

	return fed, nil
}

// GetFederate retrieves a federate by ID with thread-safety
func (f *FederationExecution) GetFederate(id int32) (*Federate, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	fed, exists := f.federates[id]
	return fed, exists
}

// RegisterSyncPoint registers a new synchronization point if the label is unique
func (f *FederationExecution) RegisterSyncPoint(label string, tag []byte, registrarID int32) (*SyncPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.syncPoints[label]; exists {
		return nil, ErrSyncPointLabelNotUnique
	}

	sp := NewSyncPoint(label, tag, registrarID)
	f.syncPoints[label] = sp
	return sp, nil
}

// AchieveSyncPoint marks a point as achieved by a federate and checks global synchronization
func (f *FederationExecution) AchieveSyncPoint(label string, federateID int32) (isFederationSynchronized bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sp, exists := f.syncPoints[label]
	if !exists {
		return false, ErrSyncPointNotFound
	}
	if sp.IsSynchronized {
		return false, nil
	}

	sp.AchievedFederates[federateID] = true

	// A federation is synchronized when ALL active federates have achieved the point
	if len(f.federates) > 0 && len(sp.AchievedFederates) >= len(f.federates) {
		sp.IsSynchronized = true
		return true, nil
	}

	return false, nil
}

// GetAllFederates returns a thread-safe slice of all active federates
func (f *FederationExecution) GetAllFederates() []*Federate {
	f.mu.RLock()
	defer f.mu.RUnlock()

	list := make([]*Federate, 0, len(f.federates))
	for _, fed := range f.federates {
		list = append(list, fed)
	}
	return list
}

// RegisterObjectInstance registers a new object instance in the federation
func (f *FederationExecution) RegisterObjectInstance(classHandle int32, name string, ownerID int32) (*ObjectInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if name != "" {
		if _, exists := f.nameToObjID[name]; exists {
			return nil, ErrObjectInstanceNameNotUnique
		}
	} else {
		name = fmt.Sprintf("HLAobjectRoot_%d", f.nextObjectHandle)
	}

	handle := f.nextObjectHandle
	f.nextObjectHandle++

	obj := &ObjectInstance{
		Handle:      handle,
		ClassHandle: classHandle,
		Name:        name,
		OwnerID:     ownerID,
		Attributes:  make(map[int32][]byte),
		UpdatedAt:   time.Now(),
	}

	f.objects[handle] = obj
	f.nameToObjID[name] = handle
	if f.stateTree != nil {
		f.stateTree.UpdateObjectState(common.ObjectInstanceHandle(handle), obj.MerkleRoot)
	}

	// Assign initial attribute ownership (direct + inherited from all ancestors) to the registering federate
	if f.ownershipMgr != nil && f.omt != nil {
		if attrHandles, ok := f.omt.GetObjectClassAttributeHandles(common.ObjectClassHandle(classHandle)); ok {
			f.ownershipMgr.RegisterInitialOwnership(common.ObjectInstanceHandle(handle), attrHandles, common.FederateHandle(ownerID))
		}
	}

	return cloneObjectInstance(obj), nil
}

// UpdateObjectAttributes updates attributes of an instance and recomputes the Merkle root
func (f *FederationExecution) UpdateObjectAttributes(objHandle int32, attrs map[int32][]byte, merkleRoot [32]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	obj, exists := f.objects[objHandle]
	if !exists {
		return common.ErrObjectInstanceNotKnown
	}

	// If Merkle audit is enabled, always compute the authoritative root deterministically via SSZ.
	// Never trust a client-supplied Merkle root blindly: if non-zero, it MUST match the authoritative root.
	if f.merkleAuditEnabled {
		attrMap := make(map[common.AttributeHandle][]byte, len(obj.Attributes)+len(attrs))
		for k, v := range obj.Attributes {
			attrMap[common.AttributeHandle(k)] = v
		}
		for k, v := range attrs {
			attrMap[common.AttributeHandle(k)] = v
		}
		authoritativeRoot := core.ComputeObjectMerkleRoot(attrMap)
		if merkleRoot != [32]byte{} && merkleRoot != authoritativeRoot {
			return ErrMerkleRootMismatch
		}

		for k, v := range attrs {
			obj.Attributes[k] = v
		}
		obj.MerkleRoot = authoritativeRoot

		if f.stateTree != nil {
			f.stateTree.UpdateObjectState(common.ObjectInstanceHandle(objHandle), authoritativeRoot)
		}
	} else {
		for k, v := range attrs {
			obj.Attributes[k] = v
		}
	}

	obj.UpdatedAt = time.Now()
	return nil
}

// DeleteObjectInstance removes the entity from simulation after validating ownership or delete privilege
func (f *FederationExecution) DeleteObjectInstance(objHandle int32, federateID int32, tag []byte) (*ObjectInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	obj, exists := f.objects[objHandle]
	if !exists {
		return nil, common.ErrObjectInstanceNotKnown
	}

	// Strict IEEE 1516-2025 standard validation:
	// Only the federate holding HLAprivilegeToDeleteObject (or initial owner if privilege not tracked) may delete
	canDelete := false
	if f.ownershipMgr != nil {
		if privOwner, isOwned := f.ownershipMgr.QueryAttributeOwnership(common.ObjectInstanceHandle(objHandle), core.HLAprivilegeToDeleteObjectHandle); isOwned {
			canDelete = (privOwner == common.FederateHandle(federateID))
		} else if f.ownershipMgr.IsAttributeDivested(common.ObjectInstanceHandle(objHandle), core.HLAprivilegeToDeleteObjectHandle) {
			canDelete = false
		} else {
			canDelete = (obj.OwnerID == federateID)
		}
	} else {
		canDelete = (obj.OwnerID == federateID)
	}
	if !canDelete {
		return nil, common.ErrDeletePrivilegeNotHeld
	}

	delete(f.objects, objHandle)
	delete(f.nameToObjID, obj.Name)

	if f.stateTree != nil {
		f.stateTree.RemoveObjectState(common.ObjectInstanceHandle(objHandle))
	}

	if f.ownershipMgr != nil {
		f.ownershipMgr.DeleteObjectOwnership(common.ObjectInstanceHandle(objHandle))
	}

	return obj, nil
}

func cloneObjectInstance(obj *ObjectInstance) *ObjectInstance {
	if obj == nil {
		return nil
	}
	attrsCopy := make(map[int32][]byte, len(obj.Attributes))
	for k, v := range obj.Attributes {
		if v != nil {
			cp := make([]byte, len(v))
			copy(cp, v)
			attrsCopy[k] = cp
		} else {
			attrsCopy[k] = nil
		}
	}
	return &ObjectInstance{
		Handle:      obj.Handle,
		ClassHandle: obj.ClassHandle,
		Name:        obj.Name,
		OwnerID:     obj.OwnerID,
		Attributes:  attrsCopy,
		MerkleRoot:  obj.MerkleRoot,
		UpdatedAt:   obj.UpdatedAt,
	}
}

// GetAllObjects returns a thread-safe snapshot of all active object instances
func (f *FederationExecution) GetAllObjects() []*ObjectInstance {
	f.mu.RLock()
	defer f.mu.RUnlock()

	list := make([]*ObjectInstance, 0, len(f.objects))
	for _, obj := range f.objects {
		list = append(list, cloneObjectInstance(obj))
	}
	return list
}

// GetObjectInstance looks up an object instance by handle and returns a defensive snapshot
func (f *FederationExecution) GetObjectInstance(handle int32) (*ObjectInstance, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	obj, exists := f.objects[handle]
	if !exists {
		return nil, false
	}
	return cloneObjectInstance(obj), true
}

// CreateRegion allocates a unique handle for a DDM spatial region
func (f *FederationExecution) CreateRegion() uint32 {
	return f.CreateRegionWithDimensions(0, nil)
}

// CreateRegionWithDimensions allocates a DDM spatial region associated with specific OMT dimension handles
func (f *FederationExecution) CreateRegionWithDimensions(ownerID int32, dims []uint32) uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()

	handle := f.nextRegionHandle
	f.nextRegionHandle++
	reg, _ := core.NewSpatialRegion(0, core.MaxCoordinateValue, 0, core.MaxCoordinateValue, 0, core.MaxCoordinateValue)
	f.regions[handle] = &reg
	if len(dims) > 0 {
		dimsCopy := make([]uint32, len(dims))
		copy(dimsCopy, dims)
		f.regionDims[handle] = dimsCopy
	}
	if ownerID > 0 {
		f.regionOwner[handle] = ownerID
	}
	return handle
}

// DeleteRegion removes a DDM spatial region and its metadata
func (f *FederationExecution) DeleteRegion(handle uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.regions[handle]; !exists {
		return errors.New("region not found")
	}
	delete(f.regions, handle)
	delete(f.regionDims, handle)
	delete(f.regionOwner, handle)
	return nil
}

// SetRegionRangeBounds sets discrete range bounds on a region dimension (supports both legacy 0..3 and OMT handles 1001+)
func (f *FederationExecution) SetRegionRangeBounds(handle uint32, dim uint32, lower, upper uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	r, exists := f.regions[handle]
	if !exists {
		return errors.New("region not found")
	}
	if lower > upper {
		return errors.New("invalid range bounds: lower > upper")
	}

	axis := -1 // 0=X, 1=Y, 2=Z
	// 1. Match against region's explicitly bound OMT dimension handles
	if dims := f.regionDims[handle]; len(dims) > 0 {
		for idx, d := range dims {
			if d == dim && idx < 3 {
				axis = idx
				break
			}
		}
	}
	// 2. Match against OMT dimension name if registered in OMT
	if axis == -1 && f.omt != nil {
		if dimDef, ok := f.omt.GetDimension(common.DimensionHandle(dim)); ok {
			switch dimDef.Name {
			case "X", "x", "DimX", "dimX":
				axis = 0
			case "Y", "y", "DimY", "dimY":
				axis = 1
			case "Z", "z", "DimZ", "dimZ":
				axis = 2
			}
		}
	}
	// 3. Backwards-compatible legacy axis handles (0/1=X, 2=Y, 3=Z)
	if axis == -1 {
		switch dim {
		case 0, 1:
			axis = 0
		case 2:
			axis = 1
		case 3:
			axis = 2
		}
	}

	switch axis {
	case 0:
		r.MinX, r.MaxX = lower, upper
	case 1:
		r.MinY, r.MaxY = lower, upper
	case 2:
		r.MinZ, r.MaxZ = lower, upper
	default:
		return errors.New("invalid dimension handle for region")
	}
	r.RecomputeMortonBounds()
	return nil
}

// SetRegionMetricBounds sets continuous ECEF metric bounds in meters for a region
func (f *FederationExecution) SetRegionMetricBounds(handle uint32, minX, maxX, minY, maxY, minZ, maxZ float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	r, exists := f.regions[handle]
	if !exists {
		return errors.New("region not found")
	}

	reg, err := core.NewSpatialRegionFromMetric(minX, maxX, minY, maxY, minZ, maxZ)
	if err != nil {
		return err
	}
	*r = reg
	return nil
}

// GetRegion returns a value copy of the DDM spatial region
func (f *FederationExecution) GetRegion(handle uint32) (core.SpatialRegion, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	r, exists := f.regions[handle]
	if !exists {
		return core.SpatialRegion{}, false
	}
	return *r, true
}

