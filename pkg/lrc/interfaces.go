package lrc

import (
	"errors"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

// ErrStateDesyncDetected is returned when local LRC state diverges from CRC Merkle root
var ErrStateDesyncDetected = errors.New("lrc: cryptographic state desync detected against CRC Merkle root")

// FederateAmbassador defines the canonical callback interface that federates implement
// to receive asynchronous notifications from the RTI (IEEE 1516 / HLA-4).
type FederateAmbassador interface {
	SynchronizationPointRegistrationSucceeded(label string)
	SynchronizationPointRegistrationFailed(label string, reason string)
	AnnounceSynchronizationPoint(label string, tag []byte)
	FederationSynchronized(label string)
	DiscoverObjectInstance(theObject int32, theObjectClass int32, objectName string)
	RemoveObjectInstance(theObject int32, tag []byte)
	ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte)
	ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte)
	TimeAdvanceGrant(time float64)
}

// DesyncAwareFederateAmbassador is an optional interface that federates can implement
// to receive immediate notifications when state desynchronization is cryptographically detected.
type DesyncAwareFederateAmbassador interface {
	FederateAmbassador
	OnStateDesyncDetected(expectedRoot, localRoot [32]byte)
}

// NullFederateAmbassador provides a default no-op implementation of FederateAmbassador,
// allowing federates to implement only the callbacks they care about.
type NullFederateAmbassador struct{}

func (n *NullFederateAmbassador) SynchronizationPointRegistrationSucceeded(label string)                  {}
func (n *NullFederateAmbassador) SynchronizationPointRegistrationFailed(label string, reason string)     {}
func (n *NullFederateAmbassador) AnnounceSynchronizationPoint(label string, tag []byte)                  {}
func (n *NullFederateAmbassador) FederationSynchronized(label string)                                    {}
func (n *NullFederateAmbassador) DiscoverObjectInstance(theObject int32, theObjectClass int32, objectName string) {}
func (n *NullFederateAmbassador) RemoveObjectInstance(theObject int32, tag []byte)                       {}
func (n *NullFederateAmbassador) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {}
func (n *NullFederateAmbassador) ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte) {}
func (n *NullFederateAmbassador) TimeAdvanceGrant(time float64)                                          {}

// RTIambassador defines the public client interface through which federates request services
// from the Local RTI Component (LRC) in accordance with IEEE 1516-2025 (HLA-4).
type RTIambassador interface {
	// Federation Management
	Connect(fedAmb FederateAmbassador, address string) error
	Disconnect() error
	CreateFederationExecution(federationName string) error
	CreateFederationExecutionWithFOM(federationName string, fomModules []string) error
	DestroyFederationExecution(federationName string) error
	JoinFederationExecution(federateName, federationName string) (int32, error)
	ResignFederationExecution() error

	// FOM / OMT Handle Resolution
	GetObjectClassHandle(className string) (int32, error)
	GetAttributeHandle(classHandle int32, attributeName string) (int32, error)
	GetDimensionHandle(dimensionName string) (int32, error)

	// Synchronization Points
	RegisterFederationSynchronizationPoint(label string, tag []byte) error
	SynchronizationPointAchieved(label string) error

	// Declaration Management (Pub/Sub)
	PublishObjectClassAttributes(objectClass int32, attributes []int32) error
	UnpublishObjectClass(objectClass int32) error
	PublishInteractionClass(interactionClass int32) error
	UnpublishInteractionClass(interactionClass int32) error
	SubscribeObjectClassAttributes(objectClass int32, attributes []int32) error
	UnsubscribeObjectClass(objectClass int32) error
	SubscribeInteractionClass(interactionClass int32) error
	UnsubscribeInteractionClass(interactionClass int32) error

	// Object Management
	RegisterObjectInstance(objectClass int32, objectName string) (int32, error)
	DeleteObjectInstance(objectHandle int32, tag []byte) error
	UpdateAttributeValues(objectHandle int32, attributes map[int32][]byte, tag []byte) error

	// Interaction Management
	SendInteraction(interactionClass int32, parameters map[int32][]byte, tag []byte) error

	// Time Management
	EnableTimeRegulation(lookahead float64) error
	DisableTimeRegulation() error
	EnableTimeConstrained() error
	DisableTimeConstrained() error
	TimeAdvanceRequest(targetTime float64) error
	NextMessageRequest(targetTime float64) error
	FlushQueueRequest(targetTime float64) error
	Heartbeat() error

	// Ownership Management
	UnconditionalAttributeOwnershipDivestiture(objectHandle int32, attributes []int32) error
	AcquireAttributeOwnership(objectHandle int32, attributes []int32) error
	QueryAttributeOwnership(objectHandle int32, attributeHandle int32) (int32, bool, error)

	// Data Distribution Management (DDM)
	CreateRegion(dimensions []int32) (uint32, error)
	SetRangeBounds(regionHandle uint32, dimension int32, lowerBound, upperBound uint32) error
	SetMetricBounds(regionHandle uint32, minX, maxX, minY, maxY, minZ, maxZ float64) error
	SubscribeObjectClassAttributesWithRegion(objectClass int32, attributes []int32, regionHandle uint32) error
	UpdateAttributeValuesWithSpatial(objectHandle int32, attributes map[int32][]byte, x, y, z float64, tag []byte) error

	// Callback Evocation
	EvokeCallback(timeout time.Duration) (bool, error)
	EvokeMultipleCallbacks(minTimeout, maxTimeout time.Duration) error
	GetCallbackQueueStatus() (length int, saturated bool, dropped uint64)
	ResetCallbackQueueSaturation()
	SetCallbackQueueCapacity(capacity int)

	// Cryptographic State Integrity & Merkle Proofs
	GetLocalStateRoot() [32]byte
	LastDesyncDetected() error
	GetObjectMerkleProof(objectHandle int32) (*common.MerkleProof, error)
	VerifyObjectInclusion(proof *common.MerkleProof, expectedRoot [32]byte) bool
}
