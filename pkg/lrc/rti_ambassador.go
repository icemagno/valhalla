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

// Connect connects the federate to the CRC executive
func (a *RTIambassadorImpl) Connect(fedAmb FederateAmbassador, address string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil && !a.client.IsClosed() {
		return ErrAlreadyConnected
	}

	if fedAmb == nil {
		fedAmb = &NullFederateAmbassador{}
	}

	client, err := NewClient(address)
	if err != nil {
		return err
	}

	a.client = client
	a.fedAmb = fedAmb
	a.address = address
	return nil
}

// Disconnect closes the connection with the CRC
func (a *RTIambassadorImpl) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil {
		return ErrNotConnected
	}

	var err error
	if !a.client.IsClosed() {
		err = a.client.Close()
	}
	a.client = nil
	a.federateID = 0
	a.federationID = 0
	return err
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
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if federationName == "" {
		return errors.New("federation name cannot be empty")
	}

	req := common.FederationExecutionPayload{FederationName: federationName}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgCreateFederation, 0, 0, payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// CreateFederationExecutionWithFOM creates a federation on the CRC with associated FOM modules
func (a *RTIambassadorImpl) CreateFederationExecutionWithFOM(federationName string, fomModules []string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
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

	resp, err := a.client.SendRequest(common.NativeMsgCreateFederation, 0, 0, payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// GetObjectClassHandle resolves the handle of an object class by its FOM name
func (a *RTIambassadorImpl) GetObjectClassHandle(className string) (int32, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, ErrNotConnected
	}

	req := common.GetObjectClassHandlePayload{ClassName: className}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgGetObjectClassHandle, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
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
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
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

	resp, err := a.client.SendRequest(common.NativeMsgGetAttributeHandle, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
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
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, ErrNotConnected
	}

	req := common.GetDimensionHandlePayload{DimensionName: dimensionName}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgGetDimensionHandle, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
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
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if federationName == "" {
		return errors.New("federation name cannot be empty")
	}

	req := common.FederationExecutionPayload{FederationName: federationName}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgDestroyFederation, 0, 0, payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// JoinFederationExecution joins the federate to a federation execution with handle assignment
func (a *RTIambassadorImpl) JoinFederationExecution(federateName, federationName string) (int32, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, ErrNotConnected
	}
	if a.federateID > 0 {
		return 0, ErrFederateAlreadyExecutionMember
	}

	joinReq := common.JoinFederationPayload{
		FederateName:   federateName,
		FederateType:   "native-hla4",
		FederationName: federationName,
	}
	payload, err := joinReq.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgJoin, 0, 0, payload, 5*time.Second)
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

	a.federateID = int32(joinResp.FederateHandle)
	a.federationID = joinResp.FederationHandle
	a.federateName = federateName
	a.federationName = federationName

	return a.federateID, nil
}

// ResignFederationExecution resigns the federate from the federation execution
func (a *RTIambassadorImpl) ResignFederationExecution() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resignReq := common.ResignFederationPayload{ResignAction: 1}
	payload, _ := resignReq.MarshalBinary()

	resp, err := a.client.SendRequest(common.NativeMsgResign, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	if err := checkNativeResponse(resp); err != nil {
		return err
	}

	a.federateID = 0
	return nil
}

// RegisterFederationSynchronizationPoint announces a new federation synchronization point
func (a *RTIambassadorImpl) RegisterFederationSynchronizationPoint(label string, tag []byte) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	syncReq := common.RegisterSyncPointPayload{
		Label: label,
		Tag:   tag,
	}
	payload, err := syncReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgRegisterSyncPoint, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// SynchronizationPointAchieved notifies the CRC that this federate achieved the synchronization point
func (a *RTIambassadorImpl) SynchronizationPointAchieved(label string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	achieveReq := common.SyncPointLabelPayload{Label: label}
	payload, err := achieveReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgAchieveSyncPoint, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

