package lrc

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

// RTIambassadorImpl implements the standard RTIambassador interface over the native Valhalla HLA-4 protocol
type RTIambassadorImpl struct {
	mu             sync.RWMutex
	client         *Client
	fedAmb         FederateAmbassador
	address        string
	federationName string
	federateName   string
	federationID   int32
	federateID     int32
	lookahead      float64
	stateTree      *common.FederationStateTree
	desyncMu       sync.RWMutex
	lastDesyncErr  error
}

// NewRTIambassador instantiates a new RTIambassador for the federate
func NewRTIambassador() RTIambassador {
	return &RTIambassadorImpl{
		stateTree: common.NewFederationStateTree(),
	}
}

// Connect connects the federate to the CRC executive without holding a.mu during network dial.
func (a *RTIambassadorImpl) Connect(fedAmb FederateAmbassador, address string) error {
	a.mu.RLock()
	alreadyConnected := a.client != nil && !a.client.IsClosed()
	a.mu.RUnlock()
	if alreadyConnected {
		return ErrAlreadyConnected
	}

	if fedAmb == nil {
		fedAmb = &NullFederateAmbassador{}
	}

	client, err := NewClient(address)
	if err != nil {
		return err
	}

	a.mu.Lock()
	if a.client != nil && !a.client.IsClosed() {
		a.mu.Unlock()
		_ = client.Close()
		return ErrAlreadyConnected
	}
	a.client = client
	a.fedAmb = fedAmb
	a.address = address
	a.mu.Unlock()
	return nil
}

func (a *RTIambassadorImpl) resetFederationStateLocked() {
	a.federateID = 0
	a.federationID = 0
	a.federateName = ""
	a.federationName = ""
	a.lookahead = 0
	a.stateTree = common.NewFederationStateTree()
	a.desyncMu.Lock()
	a.lastDesyncErr = nil
	a.desyncMu.Unlock()
}

func (a *RTIambassadorImpl) getStateTree() *common.FederationStateTree {
	a.mu.RLock()
	st := a.stateTree
	a.mu.RUnlock()
	return st
}

func (a *RTIambassadorImpl) snapshotClient() (*Client, error) {
	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()
	if client == nil || client.IsClosed() {
		return nil, ErrNotConnected
	}
	return client, nil
}

// Disconnect closes the connection with the CRC
func (a *RTIambassadorImpl) Disconnect() error {
	a.mu.Lock()
	client := a.client
	a.client = nil
	a.fedAmb = nil
	a.address = ""
	a.resetFederationStateLocked()
	a.mu.Unlock()

	if client == nil {
		return ErrNotConnected
	}

	if !client.IsClosed() {
		return client.Close()
	}
	return nil
}

func checkNativeResponse(resp nativeResponse) error {
	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return common.AsCanonicalError(errPayload.Message)
		}
		return ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgSuccessOK {
		return ErrInvalidResponse
	}
	return nil
}

// CreateFederationExecution creates or ensures existence of a federation on the CRC
func (a *RTIambassadorImpl) CreateFederationExecution(federationName string) error {
	client, err := a.snapshotClient()
	if err != nil {
		return err
	}
	if federationName == "" {
		return errors.New("federation name cannot be empty")
	}

	req := common.FederationExecutionPayload{FederationName: federationName}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgCreateFederation, 0, 0, payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// CreateFederationExecutionWithFOM creates a federation on the CRC with associated FOM modules
func (a *RTIambassadorImpl) CreateFederationExecutionWithFOM(federationName string, fomModules []string) error {
	client, err := a.snapshotClient()
	if err != nil {
		return err
	}
	if federationName == "" {
		return errors.New("federation name cannot be empty")
	}

	req := common.FederationExecutionPayload{
		FederationName: federationName,
		FOMModules:     fomModules,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgCreateFederation, 0, 0, payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// GetObjectClassHandle resolves the handle of an object class by its FOM name
func (a *RTIambassadorImpl) GetObjectClassHandle(className string) (int32, error) {
	a.mu.RLock()
	client := a.client
	fedExecID := a.federationID
	fedID := a.federateID
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return 0, ErrNotConnected
	}

	req := common.GetObjectClassHandlePayload{ClassName: className}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := client.SendRequest(common.NativeMsgGetObjectClassHandle, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, common.AsCanonicalError(errPayload.Message)
		}
		return 0, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgGetObjectClassHandleResp {
		return 0, ErrInvalidResponse
	}

	var respPayload common.HandleResponsePayload
	if err := respPayload.UnmarshalBinary(resp.Payload); err != nil {
		return 0, err
	}
	return respPayload.Handle, nil
}

// GetAttributeHandle resolves the handle of an attribute in an object class by its FOM name
func (a *RTIambassadorImpl) GetAttributeHandle(classHandle int32, attributeName string) (int32, error) {
	a.mu.RLock()
	client := a.client
	fedExecID := a.federationID
	fedID := a.federateID
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return 0, ErrNotConnected
	}

	req := common.GetAttributeHandlePayload{
		ClassHandle:   classHandle,
		AttributeName: attributeName,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := client.SendRequest(common.NativeMsgGetAttributeHandle, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, common.AsCanonicalError(errPayload.Message)
		}
		return 0, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgGetAttributeHandleResp {
		return 0, ErrInvalidResponse
	}

	var respPayload common.HandleResponsePayload
	if err := respPayload.UnmarshalBinary(resp.Payload); err != nil {
		return 0, err
	}
	return respPayload.Handle, nil
}

// GetDimensionHandle resolves the handle of a DDM dimension by its FOM name
func (a *RTIambassadorImpl) GetDimensionHandle(dimensionName string) (int32, error) {
	a.mu.RLock()
	client := a.client
	fedExecID := a.federationID
	fedID := a.federateID
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return 0, ErrNotConnected
	}

	req := common.GetDimensionHandlePayload{DimensionName: dimensionName}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := client.SendRequest(common.NativeMsgGetDimensionHandle, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, common.AsCanonicalError(errPayload.Message)
		}
		return 0, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgGetDimensionHandleResp {
		return 0, ErrInvalidResponse
	}

	var respPayload common.HandleResponsePayload
	if err := respPayload.UnmarshalBinary(resp.Payload); err != nil {
		return 0, err
	}
	return respPayload.Handle, nil
}

// DestroyFederationExecution destroys the federation execution on the CRC
func (a *RTIambassadorImpl) DestroyFederationExecution(federationName string) error {
	client, err := a.snapshotClient()
	if err != nil {
		return err
	}
	if federationName == "" {
		return errors.New("federation name cannot be empty")
	}

	req := common.FederationExecutionPayload{FederationName: federationName}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgDestroyFederation, 0, 0, payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// JoinFederationExecution joins the federate to a federation execution with the default federate type.
func (a *RTIambassadorImpl) JoinFederationExecution(federateName, federationName string) (int32, error) {
	return a.JoinFederationExecutionWithType(federateName, "native-hla4", federationName)
}

// JoinFederationExecutionWithType joins the federate to a federation execution with an explicit HLA-4 federate type.
func (a *RTIambassadorImpl) JoinFederationExecutionWithType(federateName, federateType, federationName string) (int32, error) {
	a.mu.RLock()
	client := a.client
	alreadyJoined := a.federateID > 0
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return 0, ErrNotConnected
	}
	if alreadyJoined {
		return 0, ErrFederateAlreadyExecutionMember
	}
	if federateType == "" {
		federateType = "native-hla4"
	}

	joinReq := common.JoinFederationPayload{
		FederateName:   federateName,
		FederateType:   federateType,
		FederationName: federationName,
	}
	payload, err := joinReq.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := client.SendRequest(common.NativeMsgJoin, 0, 0, payload, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, common.AsCanonicalError(errPayload.Message)
		}
		return 0, ErrInvalidResponse
	}

	if resp.Header.MsgType != common.NativeMsgJoinSuccess {
		return 0, ErrInvalidResponse
	}

	var joinResp common.JoinFederationResponsePayload
	if err := joinResp.UnmarshalBinary(resp.Payload); err != nil {
		return 0, fmt.Errorf("failed to decode join response: %w", err)
	}

	a.mu.Lock()
	a.federateID = int32(joinResp.FederateHandle)
	a.federationID = joinResp.FederationHandle
	a.federateName = federateName
	a.federationName = federationName
	a.stateTree = common.NewFederationStateTree()
	a.desyncMu.Lock()
	a.lastDesyncErr = nil
	a.desyncMu.Unlock()
	fedID := a.federateID
	a.mu.Unlock()

	return fedID, nil
}

// ResignFederationExecution resigns the federate from the federation execution using the default resign action (1).
func (a *RTIambassadorImpl) ResignFederationExecution() error {
	return a.ResignFederationExecutionWithAction(1)
}

// ResignFederationExecutionWithAction resigns the federate from the federation execution with an explicit IEEE 1516 resign action.
func (a *RTIambassadorImpl) ResignFederationExecutionWithAction(action uint8) error {
	a.mu.RLock()
	client := a.client
	fedExecID := a.federationID
	fedID := a.federateID
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return ErrNotConnected
	}
	if fedID == 0 {
		return ErrFederateNotExecutionMember
	}

	resignReq := common.ResignFederationPayload{ResignAction: action}
	payload, err := resignReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgResign, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	if err := checkNativeResponse(resp); err != nil {
		return err
	}

	a.mu.Lock()
	a.resetFederationStateLocked()
	a.mu.Unlock()
	return nil
}

// RegisterFederationSynchronizationPoint announces a new federation synchronization point
func (a *RTIambassadorImpl) RegisterFederationSynchronizationPoint(label string, tag []byte) error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	syncReq := common.RegisterSyncPointPayload{
		Label: label,
		Tag:   tag,
	}
	payload, err := syncReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgRegisterSyncPoint, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// SynchronizationPointAchieved notifies the CRC that this federate achieved the synchronization point
func (a *RTIambassadorImpl) SynchronizationPointAchieved(label string) error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	achieveReq := common.SyncPointLabelPayload{Label: label}
	payload, err := achieveReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgAchieveSyncPoint, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

