package lrc

import (
	"encoding/binary"
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
			return errors.New(errPayload.Message)
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
			return 0, errors.New(errPayload.Message)
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
			return 0, errors.New(errPayload.Message)
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
			return 0, errors.New(errPayload.Message)
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
			return 0, errors.New(errPayload.Message)
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
		failPayload, _ := common.SyncPointFailurePayload{Label: label, Reason: err.Error()}.MarshalBinary()
		a.client.EnqueueCallback(callbackItem{
			MsgType: common.NativeMsgSyncPointRegFailed,
			Payload: failPayload,
		})
		return err
	}

	if err := checkNativeResponse(resp); err != nil {
		failPayload, _ := common.SyncPointFailurePayload{Label: label, Reason: err.Error()}.MarshalBinary()
		a.client.EnqueueCallback(callbackItem{
			MsgType: common.NativeMsgSyncPointRegFailed,
			Payload: failPayload,
		})
		return err
	}

	succPayload, _ := common.SyncPointLabelPayload{Label: label}.MarshalBinary()
	a.client.EnqueueCallback(callbackItem{
		MsgType: common.NativeMsgSyncPointRegSucceeded,
		Payload: succPayload,
	})
	return nil
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

// PublishObjectClassAttributes publishes attributes of an object class
func (a *RTIambassadorImpl) PublishObjectClassAttributes(objectClass int32, attributes []int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]common.AttributeHandle, len(attributes))
	for i, h := range attributes {
		attrs[i] = common.AttributeHandle(h)
	}

	req := common.PublishObjectClassPayload{
		ClassHandle:      common.ObjectClassHandle(objectClass),
		AttributeHandles: attrs,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgPublishObjectClass, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// UnpublishObjectClass unpublishes an object class
func (a *RTIambassadorImpl) UnpublishObjectClass(objectClass int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.PublishObjectClassPayload{
		ClassHandle: common.ObjectClassHandle(objectClass),
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgUnpublishObjectClass, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// PublishInteractionClass publishes an interaction class
func (a *RTIambassadorImpl) PublishInteractionClass(interactionClass int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.InteractionClassPayload{
		ClassHandle: common.InteractionClassHandle(interactionClass),
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgPublishInteractionClass, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// UnpublishInteractionClass unpublishes an interaction class
func (a *RTIambassadorImpl) UnpublishInteractionClass(interactionClass int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.InteractionClassPayload{
		ClassHandle: common.InteractionClassHandle(interactionClass),
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgUnpublishInteractionClass, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// SubscribeObjectClassAttributes subscribes to attributes of an object class
func (a *RTIambassadorImpl) SubscribeObjectClassAttributes(objectClass int32, attributes []int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]common.AttributeHandle, len(attributes))
	for i, h := range attributes {
		attrs[i] = common.AttributeHandle(h)
	}

	req := common.SubscribeObjectClassPayload{
		ClassHandle:      common.ObjectClassHandle(objectClass),
		AttributeHandles: attrs,
		Active:           true,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSubscribeObjectClassAttrs, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// UnsubscribeObjectClass unsubscribes from an object class
func (a *RTIambassadorImpl) UnsubscribeObjectClass(objectClass int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.PublishObjectClassPayload{
		ClassHandle: common.ObjectClassHandle(objectClass),
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgUnsubscribeObjectClass, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// SubscribeInteractionClass subscribes to an interaction class
func (a *RTIambassadorImpl) SubscribeInteractionClass(interactionClass int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.SubscribeInteractionClassPayload{
		ClassHandle: common.InteractionClassHandle(interactionClass),
		Active:      true,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSubscribeInteractionClass, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// UnsubscribeInteractionClass unsubscribes from an interaction class
func (a *RTIambassadorImpl) UnsubscribeInteractionClass(interactionClass int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.InteractionClassPayload{
		ClassHandle: common.InteractionClassHandle(interactionClass),
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgUnsubscribeInteractionClass, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// RegisterObjectInstance registers a new object instance in the simulation model
func (a *RTIambassadorImpl) RegisterObjectInstance(objectClass int32, objectName string) (int32, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, ErrNotConnected
	}
	if a.federateID == 0 {
		return 0, ErrFederateNotExecutionMember
	}

	regReq := common.RegisterObjectInstancePayload{
		ClassHandle: common.ObjectClassHandle(objectClass),
		ObjectName:  objectName,
	}
	payload, err := regReq.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgRegisterObject, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, errors.New(errPayload.Message)
		}
		return 0, ErrInvalidResponse
	}

	if resp.Header.MsgType != common.NativeMsgSuccessOK || len(resp.Payload) < 4 {
		return 0, ErrInvalidResponse
	}

	objHandle := int32(binary.LittleEndian.Uint32(resp.Payload[0:4]))
	a.stateTree.UpdateObjectState(common.ObjectInstanceHandle(objHandle), [32]byte{})
	return objHandle, nil
}

// DeleteObjectInstance requests deletion of an object instance from the federation
func (a *RTIambassadorImpl) DeleteObjectInstance(objectHandle int32, tag []byte) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	delReq := common.DeleteObjectInstancePayload{
		ObjectHandle: common.ObjectInstanceHandle(objectHandle),
		Tag:          tag,
	}
	payload, err := delReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgDeleteObjectInstance, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	if err := checkNativeResponse(resp); err != nil {
		return err
	}
	a.stateTree.RemoveObjectState(common.ObjectInstanceHandle(objectHandle))
	return nil
}

// UpdateAttributeValues dispatches updated attribute values to the federation
func (a *RTIambassadorImpl) UpdateAttributeValues(objectHandle int32, attributes map[int32][]byte, tag []byte) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrMap := make(map[common.AttributeHandle][]byte, len(attributes))
	for k, v := range attributes {
		attrMap[common.AttributeHandle(k)] = v
	}

	uavReq := common.UpdateAttributesPayload{
		ObjectHandle: common.ObjectInstanceHandle(objectHandle),
		Attributes:   attrMap,
		Tag:          tag,
		HasTime:      false,
	}
	payload, err := uavReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgUpdateAttributes, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	if err := checkNativeResponse(resp); err != nil {
		return err
	}

	for k, v := range attrMap {
		a.stateTree.UpdateEntityAttribute(common.ObjectInstanceHandle(objectHandle), k, v)
	}
	if (resp.Header.Flags & common.FlagNativeMerkle) != 0 {
		localRoot := a.stateTree.GetGlobalRoot()
		if localRoot != resp.Header.MerkleRoot {
			desyncErr := fmt.Errorf("%w: CRC root %x != local root %x", ErrStateDesyncDetected, resp.Header.MerkleRoot, localRoot)
			a.desyncMu.Lock()
			a.lastDesyncErr = desyncErr
			a.desyncMu.Unlock()
			if desyncAmb, ok := a.fedAmb.(DesyncAwareFederateAmbassador); ok {
				desyncAmb.OnStateDesyncDetected(resp.Header.MerkleRoot, localRoot)
			}
		}
	}

	return nil
}

// SendInteraction dispatches an interaction with parameters to the federation
func (a *RTIambassadorImpl) SendInteraction(interactionClass int32, parameters map[int32][]byte, tag []byte) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	paramMap := make(map[common.ParameterHandle][]byte, len(parameters))
	for k, v := range parameters {
		paramMap[common.ParameterHandle(k)] = v
	}

	interReq := common.SendInteractionPayload{
		ClassHandle: common.InteractionClassHandle(interactionClass),
		Parameters:  paramMap,
		Tag:         tag,
		HasTime:     false,
	}
	payload, err := interReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSendInteraction, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	return checkNativeResponse(resp)
}

// EnableTimeRegulation enables time regulation with the specified lookahead
func (a *RTIambassadorImpl) EnableTimeRegulation(lookahead float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}
	if lookahead < 0 {
		return common.ErrInvalidLookahead
	}

	timeReq := common.TimeFloat64Payload{Time: lookahead}
	payload, err := timeReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgEnableTimeRegulation, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return errors.New(errPayload.Message)
		}
		return ErrInvalidResponse
	}

	if resp.Header.MsgType != common.NativeMsgTimeRegulationEnabled {
		return ErrInvalidResponse
	}

	a.lookahead = lookahead
	return nil
}

// DisableTimeRegulation disables time regulation for the federate
func (a *RTIambassadorImpl) DisableTimeRegulation() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resp, err := a.client.SendRequest(common.NativeMsgDisableTimeRegulation, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// EnableTimeConstrained enables time constrained state for the federate
func (a *RTIambassadorImpl) EnableTimeConstrained() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resp, err := a.client.SendRequest(common.NativeMsgEnableTimeConstrained, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
	if err != nil {
		return err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return errors.New(errPayload.Message)
		}
		return ErrInvalidResponse
	}

	if resp.Header.MsgType != common.NativeMsgTimeConstrainedEnabled {
		return ErrInvalidResponse
	}

	return nil
}

// DisableTimeConstrained disables time constrained state for the federate
func (a *RTIambassadorImpl) DisableTimeConstrained() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resp, err := a.client.SendRequest(common.NativeMsgDisableTimeConstrained, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// TimeAdvanceRequest requests advance of logical simulation time
func (a *RTIambassadorImpl) TimeAdvanceRequest(targetTime float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	tarReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := tarReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgTimeAdvanceRequest, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// NextMessageRequest requests advance to the next message timestamp or targetTime
func (a *RTIambassadorImpl) NextMessageRequest(targetTime float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	nmrReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := nmrReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgNextMessageRequest, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// FlushQueueRequest requests advance with delivery of all queued TSO messages
func (a *RTIambassadorImpl) FlushQueueRequest(targetTime float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	flushReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := flushReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgFlushQueueRequest, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// EvokeCallback delivers one pending callback to the FederateAmbassador if available
func (a *RTIambassadorImpl) EvokeCallback(timeout time.Duration) (bool, error) {
	a.mu.RLock()
	client := a.client
	fedAmb := a.fedAmb
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return false, ErrNotConnected
	}

	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}

	select {
	case cb := <-client.callbacks:
		a.dispatchCallback(cb, fedAmb)
		return true, nil
	case <-timer:
		return false, nil
	case <-client.closedChan:
		return false, ErrConnectionClosed
	}
}

// EvokeMultipleCallbacks repeatedly processes available callbacks up to maxTimeout
func (a *RTIambassadorImpl) EvokeMultipleCallbacks(minTimeout, maxTimeout time.Duration) error {
	deadline := time.Now().Add(maxTimeout)
	for time.Now().Before(deadline) {
		evoked, err := a.EvokeCallback(minTimeout)
		if err != nil {
			return err
		}
		if !evoked {
			break
		}
	}
	return nil
}

func (a *RTIambassadorImpl) dispatchCallback(cb callbackItem, fedAmb FederateAmbassador) {
	if fedAmb == nil {
		return
	}

	switch cb.MsgType {
	case common.NativeMsgSyncPointRegSucceeded:
		var succPayload common.SyncPointLabelPayload
		if err := succPayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.SynchronizationPointRegistrationSucceeded(succPayload.Label)
		} else {
			fedAmb.SynchronizationPointRegistrationSucceeded(string(cb.Payload))
		}

	case common.NativeMsgSyncPointRegFailed:
		var failPayload common.SyncPointFailurePayload
		if err := failPayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.SynchronizationPointRegistrationFailed(failPayload.Label, failPayload.Reason)
		} else {
			var labelPayload common.SyncPointLabelPayload
			if err2 := labelPayload.UnmarshalBinary(cb.Payload); err2 == nil {
				fedAmb.SynchronizationPointRegistrationFailed(labelPayload.Label, "synchronization point registration failed")
			} else {
				fedAmb.SynchronizationPointRegistrationFailed(string(cb.Payload), "synchronization point registration failed")
			}
		}

	case common.NativeMsgAnnounceSyncPoint:
		var annPayload common.RegisterSyncPointPayload
		if err := annPayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.AnnounceSynchronizationPoint(annPayload.Label, annPayload.Tag)
		}

	case common.NativeMsgFederationSynchronized:
		var syncPayload common.SyncPointLabelPayload
		if err := syncPayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.FederationSynchronized(syncPayload.Label)
		} else {
			fedAmb.FederationSynchronized(string(cb.Payload))
		}

	case common.NativeMsgDiscoverObject:
		var discPayload common.DiscoverObjectInstancePayload
		if err := discPayload.UnmarshalBinary(cb.Payload); err == nil {
			a.stateTree.UpdateObjectState(discPayload.ObjectHandle, [32]byte{})
			fedAmb.DiscoverObjectInstance(int32(discPayload.ObjectHandle), int32(discPayload.ClassHandle), discPayload.ObjectName)
		}

	case common.NativeMsgRemoveObjectInstance:
		var remPayload common.RemoveObjectInstancePayload
		if err := remPayload.UnmarshalBinary(cb.Payload); err == nil {
			a.stateTree.RemoveObjectState(remPayload.ObjectHandle)
			fedAmb.RemoveObjectInstance(int32(remPayload.ObjectHandle), remPayload.Tag)
		}

	case common.NativeMsgReflectAttributes:
		var uavPayload common.UpdateAttributesPayload
		if err := uavPayload.UnmarshalBinary(cb.Payload); err == nil {
			attrs := make(map[int32][]byte, len(uavPayload.Attributes))
			for k, v := range uavPayload.Attributes {
				attrs[int32(k)] = v
				a.stateTree.UpdateEntityAttribute(uavPayload.ObjectHandle, k, v)
			}
			fedAmb.ReflectAttributeValues(int32(uavPayload.ObjectHandle), attrs, uavPayload.Tag)

			if cb.HasMerkle {
				localRoot := a.stateTree.GetGlobalRoot()
				if localRoot != cb.MerkleRoot {
					desyncErr := fmt.Errorf("%w: CRC root %x != local root %x", ErrStateDesyncDetected, cb.MerkleRoot, localRoot)
					a.desyncMu.Lock()
					a.lastDesyncErr = desyncErr
					a.desyncMu.Unlock()
					if desyncAmb, ok := fedAmb.(DesyncAwareFederateAmbassador); ok {
						desyncAmb.OnStateDesyncDetected(cb.MerkleRoot, localRoot)
					}
				}
			}
		}

	case common.NativeMsgReceiveInteraction:
		var interPayload common.SendInteractionPayload
		if err := interPayload.UnmarshalBinary(cb.Payload); err == nil {
			params := make(map[int32][]byte, len(interPayload.Parameters))
			for k, v := range interPayload.Parameters {
				params[int32(k)] = v
			}
			fedAmb.ReceiveInteraction(int32(interPayload.ClassHandle), params, interPayload.Tag)
		}

	case common.NativeMsgTimeAdvanceGrant:
		var timePayload common.TimeFloat64Payload
		if err := timePayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.TimeAdvanceGrant(timePayload.Time)
		}
	}
}

// UnconditionalAttributeOwnershipDivestiture unconditionally divests ownership of the specified attributes
func (a *RTIambassadorImpl) UnconditionalAttributeOwnershipDivestiture(objectHandle int32, attributes []int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]uint32, len(attributes))
	for i, h := range attributes {
		attrs[i] = uint32(h)
	}

	req := common.DivestOwnershipPayload{
		ObjectHandle: uint32(objectHandle),
		Attributes:   attrs,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgDivestOwnership, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// AcquireAttributeOwnership acquires ownership of unowned or divested attributes
func (a *RTIambassadorImpl) AcquireAttributeOwnership(objectHandle int32, attributes []int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]uint32, len(attributes))
	for i, h := range attributes {
		attrs[i] = uint32(h)
	}

	req := common.AcquireOwnershipPayload{
		ObjectHandle: uint32(objectHandle),
		Attributes:   attrs,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgAcquireOwnership, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// QueryAttributeOwnership queries the current owner of an object instance attribute
func (a *RTIambassadorImpl) QueryAttributeOwnership(objectHandle int32, attributeHandle int32) (int32, bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, false, ErrNotConnected
	}
	if a.federateID == 0 {
		return 0, false, ErrFederateNotExecutionMember
	}

	req := common.QueryOwnershipPayload{
		ObjectHandle:    uint32(objectHandle),
		AttributeHandle: uint32(attributeHandle),
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, false, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgQueryAttributeOwnership, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return 0, false, err
	}
	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, false, errors.New(errPayload.Message)
		}
		return 0, false, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgAttributeOwnershipResponse {
		return 0, false, ErrInvalidResponse
	}

	var respPayload common.QueryOwnershipResponsePayload
	if err := respPayload.UnmarshalBinary(resp.Payload); err != nil {
		return 0, false, err
	}
	return int32(respPayload.OwnerHandle), respPayload.IsOwned, nil
}

// CreateRegion creates a spatial region for Data Distribution Management (DDM)
func (a *RTIambassadorImpl) CreateRegion(dimensions []int32) (uint32, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, ErrNotConnected
	}
	if a.federateID == 0 {
		return 0, ErrFederateNotExecutionMember
	}

	dims := make([]uint32, len(dimensions))
	for i, d := range dimensions {
		dims[i] = uint32(d)
	}

	req := common.CreateRegionPayload{
		Dimensions: dims,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgCreateRegion, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return 0, err
	}
	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, errors.New(errPayload.Message)
		}
		return 0, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgRegionCreated {
		return 0, ErrInvalidResponse
	}

	var respPayload common.CreateRegionResponsePayload
	if err := respPayload.UnmarshalBinary(resp.Payload); err != nil {
		return 0, err
	}
	return respPayload.RegionHandle, nil
}

// SetRangeBounds sets discrete range bounds on a region dimension
func (a *RTIambassadorImpl) SetRangeBounds(regionHandle uint32, dimension int32, lowerBound, upperBound uint32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.SetRangeBoundsPayload{
		RegionHandle:    regionHandle,
		DimensionHandle: uint32(dimension),
		LowerBound:      lowerBound,
		UpperBound:      upperBound,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSetRangeBounds, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// SetMetricBounds sets continuous ECEF metric bounds in meters for a DDM spatial region
func (a *RTIambassadorImpl) SetMetricBounds(regionHandle uint32, minX, maxX, minY, maxY, minZ, maxZ float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.SetMetricBoundsPayload{
		RegionHandle: regionHandle,
		MinX:         minX,
		MaxX:         maxX,
		MinY:         minY,
		MaxY:         maxY,
		MinZ:         minZ,
		MaxZ:         maxZ,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSetMetricBounds, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// SubscribeObjectClassAttributesWithRegion subscribes to class attributes bound to a DDM spatial region
func (a *RTIambassadorImpl) SubscribeObjectClassAttributesWithRegion(objectClass int32, attributes []int32, regionHandle uint32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]uint32, len(attributes))
	for i, h := range attributes {
		attrs[i] = uint32(h)
	}

	req := common.SubscribeWithRegionPayload{
		ObjectClass:  uint32(objectClass),
		Attributes:   attrs,
		RegionHandle: regionHandle,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSubscribeWithRegion, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// UpdateAttributeValuesWithSpatial dispatches updated attribute values with metric (ECEF) spatial coordinates
func (a *RTIambassadorImpl) UpdateAttributeValuesWithSpatial(objectHandle int32, attributes map[int32][]byte, x, y, z float64, tag []byte) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrMap := make(map[common.AttributeHandle][]byte, len(attributes))
	for k, v := range attributes {
		attrMap[common.AttributeHandle(k)] = v
	}

	req := common.UpdateAttributesSpatialPayload{
		ObjectHandle: common.ObjectInstanceHandle(objectHandle),
		Attributes:   attrMap,
		Tag:          tag,
		X:            x,
		Y:            y,
		Z:            z,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgUpdateAttributesSpatial, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	if err := checkNativeResponse(resp); err != nil {
		return err
	}

	for k, v := range attrMap {
		a.stateTree.UpdateEntityAttribute(common.ObjectInstanceHandle(objectHandle), k, v)
	}
	if (resp.Header.Flags & common.FlagNativeMerkle) != 0 {
		localRoot := a.stateTree.GetGlobalRoot()
		if localRoot != resp.Header.MerkleRoot {
			desyncErr := fmt.Errorf("%w: CRC root %x != local root %x", ErrStateDesyncDetected, resp.Header.MerkleRoot, localRoot)
			a.desyncMu.Lock()
			a.lastDesyncErr = desyncErr
			a.desyncMu.Unlock()
			if desyncAmb, ok := a.fedAmb.(DesyncAwareFederateAmbassador); ok {
				desyncAmb.OnStateDesyncDetected(resp.Header.MerkleRoot, localRoot)
			}
		}
	}
	return nil
}

// GetLocalStateRoot returns the deterministic 32-byte Merkle root of the local LRC state
func (a *RTIambassadorImpl) GetLocalStateRoot() [32]byte {
	return a.stateTree.GetGlobalRoot()
}

// LastDesyncDetected returns the last state desynchronization error, if any
func (a *RTIambassadorImpl) LastDesyncDetected() error {
	a.desyncMu.RLock()
	defer a.desyncMu.RUnlock()
	return a.lastDesyncErr
}

// CorruptLocalStateForTesting deliberately injects an invalid attribute into the local state
// to test adversarial desync detection against the CRC in integration tests.
func (a *RTIambassadorImpl) CorruptLocalStateForTesting(objectHandle common.ObjectInstanceHandle, attributeHandle common.AttributeHandle, val []byte) {
	a.stateTree.UpdateEntityAttribute(objectHandle, attributeHandle, val)
}

// GetObjectMerkleProof requests a cryptographic Merkle inclusion proof for an object from the CRC
func (a *RTIambassadorImpl) GetObjectMerkleProof(objectHandle int32) (*common.MerkleProof, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return nil, ErrNotConnected
	}
	if a.federateID == 0 {
		return nil, ErrFederateNotExecutionMember
	}

	reqPayload, err := common.GetMerkleProofPayload{
		ObjectHandle: common.ObjectInstanceHandle(objectHandle),
	}.MarshalBinary()
	if err != nil {
		return nil, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgGetMerkleProof, uint32(a.federationID), uint32(a.federateID), reqPayload, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return nil, errors.New(errPayload.Message)
		}
		return nil, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgGetMerkleProofResp {
		return nil, ErrInvalidResponse
	}

	var proofResp common.MerkleProofResponsePayload
	if err := proofResp.UnmarshalBinary(resp.Payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merkle proof response: %w", err)
	}

	return &common.MerkleProof{
		LeafHash:   proofResp.LeafHash,
		AuditPath:  proofResp.AuditPath,
		LeafIndex:  proofResp.LeafIndex,
		GlobalRoot: proofResp.GlobalRoot,
	}, nil
}

// VerifyObjectInclusion mathematically validates a Merkle proof against a trusted expectedRoot
func (a *RTIambassadorImpl) VerifyObjectInclusion(proof *common.MerkleProof, expectedRoot [32]byte) bool {
	return common.VerifyMerkleProof(proof, expectedRoot)
}

