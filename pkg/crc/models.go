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
	ProtocolNative  = common.ProtocolNative
	ProtocolPortico = common.ProtocolPortico
	ProtocolWeb     = common.ProtocolWeb
)

// Session defines the transport interface for asynchronous message dispatch to a federate
type Session interface {
	FederateID() int32
	Protocol() ProtocolType
	SendNotification(msgType uint8, payload []byte) error
	Close() error
}

// Federate represents a participant node in a federation execution on the CRC
type Federate struct {
	ID           int32
	Name         string
	FederateType string
	Protocol     ProtocolType
	Session      Session
	JoinedAt     time.Time

	// Time Management State
	CurrentTime   float64
	Lookahead     float64
	IsRegulating  bool
	IsConstrained bool
	IsAdvancing   bool
	TargetTime    float64
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

	omt          *core.OMTRegistry
	router       *core.InterestMatcher
	stateTree    *core.FederationStateTree
	timeMgr      *core.TimeManager
	ownershipMgr *core.OwnershipManager
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
		omt:                omt,
		router:             router,
		stateTree:          stateTree,
		timeMgr:            timeMgr,
		ownershipMgr:       ownershipMgr,
	}
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
		Lookahead:    0.1,
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
	defer f.mu.Unlock()

	fed, exists := f.federates[id]
	if !exists {
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

	// Remove federate from time manager
	if f.timeMgr != nil {
		f.timeMgr.UnregisterFederate(common.FederateHandle(id))
	}

	// Clear attribute ownership held by federate
	if f.ownershipMgr != nil {
		f.ownershipMgr.RemoveFederate(common.FederateHandle(id))
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

	// Assign initial attribute ownership to the registering federate
	if f.ownershipMgr != nil && f.omt != nil {
		if cls, ok := f.omt.GetObjectClass(common.ObjectClassHandle(classHandle)); ok {
			var attrHandles []common.AttributeHandle
			for _, a := range cls.Attributes {
				attrHandles = append(attrHandles, a.Handle)
			}
			f.ownershipMgr.RegisterInitialOwnership(common.ObjectInstanceHandle(handle), attrHandles, common.FederateHandle(ownerID))
		}
	}

	return obj, nil
}

// UpdateObjectAttributes updates attributes of an instance and recomputes the Merkle root
func (f *FederationExecution) UpdateObjectAttributes(objHandle int32, attrs map[int32][]byte, merkleRoot [32]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	obj, exists := f.objects[objHandle]
	if !exists {
		return common.ErrObjectInstanceNotKnown
	}

	for k, v := range attrs {
		obj.Attributes[k] = v
	}

	// If Merkle root was not provided, compute deterministically via SSZ
	if merkleRoot == [32]byte{} {
		attrMap := make(map[common.AttributeHandle][]byte, len(obj.Attributes))
		for k, v := range obj.Attributes {
			attrMap[common.AttributeHandle(k)] = v
		}
		merkleRoot = core.ComputeObjectMerkleRoot(attrMap)
	}

	obj.MerkleRoot = merkleRoot
	obj.UpdatedAt = time.Now()

	if f.stateTree != nil {
		f.stateTree.UpdateObjectState(common.ObjectInstanceHandle(objHandle), merkleRoot)
	}

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
	// Only registering owner OR federate holding delete privilege can delete instance
	if obj.OwnerID != federateID {
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

// GetAllObjects returns a thread-safe snapshot of all active object instances
func (f *FederationExecution) GetAllObjects() []*ObjectInstance {
	f.mu.RLock()
	defer f.mu.RUnlock()

	list := make([]*ObjectInstance, 0, len(f.objects))
	for _, obj := range f.objects {
		list = append(list, obj)
	}
	return list
}

// GetObjectInstance looks up an object instance by handle
func (f *FederationExecution) GetObjectInstance(handle int32) (*ObjectInstance, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	obj, exists := f.objects[handle]
	return obj, exists
}

// CreateRegion allocates a unique handle for a DDM spatial region
func (f *FederationExecution) CreateRegion() uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()

	handle := f.nextRegionHandle
	f.nextRegionHandle++
	reg, _ := core.NewSpatialRegion(0, core.MaxCoordinateValue, 0, core.MaxCoordinateValue, 0, core.MaxCoordinateValue)
	f.regions[handle] = &reg
	return handle
}

// SetRegionRangeBounds sets discrete range bounds on a region dimension
func (f *FederationExecution) SetRegionRangeBounds(handle uint32, dim uint32, lower, upper uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	r, exists := f.regions[handle]
	if !exists {
		return errors.New("region not found")
	}

	switch dim {
	case 1, 0:
		r.MinX, r.MaxX = lower, upper
	case 2:
		r.MinY, r.MaxY = lower, upper
	case 3:
		r.MinZ, r.MaxZ = lower, upper
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

