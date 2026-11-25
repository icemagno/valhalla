package lrc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

// RTIambassadorImpl implementa a interface padrão RTIambassador sobre o protocolo nativo Valhalla (HLA-4)
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
}

// NewRTIambassador instancia um novo RTIambassador para o federado
func NewRTIambassador() RTIambassador {
	return &RTIambassadorImpl{}
}

// Connect conecta o federado ao servidor executivo CRC
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

// Disconnect encerra a conexão com o CRC
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

// CreateFederationExecution cria ou assegura a existência de uma federação no CRC
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

// DestroyFederationExecution remove a federação do CRC
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

// JoinFederationExecution ingressa o federado em uma federação com atribuição de handle
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

// ResignFederationExecution desingressa o federado da federação
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

// RegisterFederationSynchronizationPoint anuncia um novo ponto de sincronização
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

// SynchronizationPointAchieved notifica o CRC que este federado conquistou o ponto
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

// PublishObjectClassAttributes publica atributos de uma classe de objeto
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

// UnpublishObjectClass cancela a publicação de uma classe de objeto
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

// PublishInteractionClass publica uma classe de interação
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

// UnpublishInteractionClass cancela a publicação de uma classe de interação
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

// SubscribeObjectClassAttributes assina atributos de uma classe de objeto
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

// UnsubscribeObjectClass cancela a assinatura de uma classe de objeto
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

// SubscribeInteractionClass assina uma classe de interação
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

// UnsubscribeInteractionClass cancela a assinatura de uma classe de interação
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

// RegisterObjectInstance registra uma nova instância de objeto no modelo de simulação
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

	objHandle := int32(binary.BigEndian.Uint32(resp.Payload[0:4]))
	return objHandle, nil
}

// UpdateAttributeValues despacha valores de atributos atualizados para a federação
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

	return checkNativeResponse(resp)
}

// SendInteraction despacha uma interação com parâmetros para a federação
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

// EnableTimeRegulation habilita a regulação de tempo com lookahead
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

// EnableTimeConstrained habilita a restrição de tempo do federado
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

// TimeAdvanceRequest solicita avanço no tempo lógico da simulação
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

// EvokeCallback entrega um callback pendente à FederateAmbassador se houver
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

// EvokeMultipleCallbacks processa continuamente callbacks disponíveis até o limite de tempo
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
			fedAmb.DiscoverObjectInstance(int32(discPayload.ObjectHandle), int32(discPayload.ClassHandle), discPayload.ObjectName)
		}

	case common.NativeMsgReflectAttributes:
		var uavPayload common.UpdateAttributesPayload
		if err := uavPayload.UnmarshalBinary(cb.Payload); err == nil {
			attrs := make(map[int32][]byte, len(uavPayload.Attributes))
			for k, v := range uavPayload.Attributes {
				attrs[int32(k)] = v
			}
			fedAmb.ReflectAttributeValues(int32(uavPayload.ObjectHandle), attrs, uavPayload.Tag)
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
