package lrc

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

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
			return 0, common.AsCanonicalError(errPayload.Message)
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
	client := a.client
	fedID := a.federationID
	federateID := a.federateID
	fedAmb := a.fedAmb
	stateTree := a.stateTree
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return ErrNotConnected
	}
	if federateID == 0 {
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

	resp, err := client.SendRequest(common.NativeMsgUpdateAttributes, uint32(fedID), uint32(federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	if err := checkNativeResponse(resp); err != nil {
		return err
	}

	for k, v := range attrMap {
		stateTree.UpdateEntityAttribute(common.ObjectInstanceHandle(objectHandle), k, v)
	}
	if (resp.Header.Flags & common.FlagNativeMerkle) != 0 {
		localRoot := stateTree.GetGlobalRoot()
		if localRoot != resp.Header.MerkleRoot {
			desyncErr := fmt.Errorf("%w: CRC root %x != local root %x", ErrStateDesyncDetected, resp.Header.MerkleRoot, localRoot)
			a.desyncMu.Lock()
			a.lastDesyncErr = desyncErr
			a.desyncMu.Unlock()
			if desyncAmb, ok := fedAmb.(DesyncAwareFederateAmbassador); ok {
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

