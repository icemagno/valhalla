package lrc

import (
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/crc"
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

// CreateFederationExecution cria ou assegura a existência de uma federação no CRC
func (a *RTIambassadorImpl) CreateFederationExecution(federationName string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	// O Valhalla CRC assegura e instancia federações sob demanda de forma concorrente
	return nil
}

// DestroyFederationExecution remove a federação do CRC
func (a *RTIambassadorImpl) DestroyFederationExecution(federationName string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	return nil
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

	payload := []byte(federateName)
	resp, err := a.client.SendRequest(crc.NativeMsgJoin, 0, 0, payload, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if resp.Header.MsgType != crc.NativeMsgJoinSuccess || len(resp.Payload) < 4 {
		return 0, ErrInvalidResponse
	}

	fedID := int32(binary.BigEndian.Uint32(resp.Payload[0:4]))
	a.federateID = fedID
	a.federationID = int32(resp.Header.FedID)
	a.federateName = federateName
	a.federationName = federationName

	return fedID, nil
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

	_, err := a.client.SendRequest(crc.NativeMsgResign, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
	if err != nil {
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

	_, err := a.client.SendRequest(crc.NativeMsgRegisterSyncPoint, uint32(a.federationID), uint32(a.federateID), []byte(label), 5*time.Second)
	return err
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

	_, err := a.client.SendRequest(crc.NativeMsgAchieveSyncPoint, uint32(a.federationID), uint32(a.federateID), []byte(label), 5*time.Second)
	return err
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

	payload := make([]byte, 6+len(objectName))
	binary.BigEndian.PutUint32(payload[0:4], uint32(objectClass))
	binary.BigEndian.PutUint16(payload[4:6], uint16(len(objectName)))
	copy(payload[6:], []byte(objectName))

	resp, err := a.client.SendRequest(crc.NativeMsgRegisterObject, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if len(resp.Payload) < 4 {
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

	// Payload: objHandle (4B) + attrCount (2B) + [attrHandle (4B) + len (4B) + value]...
	totalSize := 6
	for _, val := range attributes {
		totalSize += 8 + len(val)
	}

	buf := make([]byte, totalSize)
	binary.BigEndian.PutUint32(buf[0:4], uint32(objectHandle))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(attributes)))

	offset := 6
	for handle, val := range attributes {
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(handle))
		binary.BigEndian.PutUint32(buf[offset+4:offset+8], uint32(len(val)))
		copy(buf[offset+8:offset+8+len(val)], val)
		offset += 8 + len(val)
	}

	_, err := a.client.SendRequest(crc.NativeMsgUpdateAttributes, uint32(a.federationID), uint32(a.federateID), buf, 5*time.Second)
	return err
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

	totalSize := 6
	for _, val := range parameters {
		totalSize += 8 + len(val)
	}

	buf := make([]byte, totalSize)
	binary.BigEndian.PutUint32(buf[0:4], uint32(interactionClass))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(parameters)))

	offset := 6
	for handle, val := range parameters {
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(handle))
		binary.BigEndian.PutUint32(buf[offset+4:offset+8], uint32(len(val)))
		copy(buf[offset+8:offset+8+len(val)], val)
		offset += 8 + len(val)
	}

	_, err := a.client.SendRequest(crc.NativeMsgSendInteraction, uint32(a.federationID), uint32(a.federateID), buf, 5*time.Second)
	return err
}

// EnableTimeRegulation habilita a regulação de tempo com lookahead
func (a *RTIambassadorImpl) EnableTimeRegulation(lookahead float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lookahead = lookahead
	return nil
}

// EnableTimeConstrained habilita a restrição de tempo do federado
func (a *RTIambassadorImpl) EnableTimeConstrained() error {
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

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(targetTime))

	_, err := a.client.SendRequest(crc.NativeMsgTimeAdvanceRequest, uint32(a.federationID), uint32(a.federateID), buf[:], 5*time.Second)
	return err
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
	case crc.NativeMsgFederationSynchronized:
		fedAmb.FederationSynchronized(string(cb.Payload))

	case crc.NativeMsgDiscoverObject:
		if len(cb.Payload) >= 10 {
			objHandle := int32(binary.BigEndian.Uint32(cb.Payload[0:4]))
			classHandle := int32(binary.BigEndian.Uint32(cb.Payload[4:8]))
			nameLen := int(binary.BigEndian.Uint16(cb.Payload[8:10]))
			var name string
			if len(cb.Payload) >= 10+nameLen {
				name = string(cb.Payload[10 : 10+nameLen])
			}
			fedAmb.DiscoverObjectInstance(objHandle, classHandle, name)
		}

	case crc.NativeMsgReflectAttributes:
		if len(cb.Payload) >= 6 {
			objHandle := int32(binary.BigEndian.Uint32(cb.Payload[0:4]))
			count := int(binary.BigEndian.Uint16(cb.Payload[4:6]))
			attrs := make(map[int32][]byte, count)

			offset := 6
			for i := 0; i < count && offset+8 <= len(cb.Payload); i++ {
				attrHandle := int32(binary.BigEndian.Uint32(cb.Payload[offset : offset+4]))
				valLen := int(binary.BigEndian.Uint32(cb.Payload[offset+4 : offset+8]))
				offset += 8
				if offset+valLen <= len(cb.Payload) {
					attrs[attrHandle] = append([]byte(nil), cb.Payload[offset:offset+valLen]...)
					offset += valLen
				}
			}
			fedAmb.ReflectAttributeValues(objHandle, attrs, nil)
		}

	case crc.NativeMsgReceiveInteraction:
		if len(cb.Payload) >= 6 {
			classHandle := int32(binary.BigEndian.Uint32(cb.Payload[0:4]))
			count := int(binary.BigEndian.Uint16(cb.Payload[4:6]))
			params := make(map[int32][]byte, count)

			offset := 6
			for i := 0; i < count && offset+8 <= len(cb.Payload); i++ {
				paramHandle := int32(binary.BigEndian.Uint32(cb.Payload[offset : offset+4]))
				valLen := int(binary.BigEndian.Uint32(cb.Payload[offset+4 : offset+8]))
				offset += 8
				if offset+valLen <= len(cb.Payload) {
					params[paramHandle] = append([]byte(nil), cb.Payload[offset:offset+valLen]...)
					offset += valLen
				}
			}
			fedAmb.ReceiveInteraction(classHandle, params, nil)
		}

	case crc.NativeMsgTimeAdvanceGrant:
		if len(cb.Payload) >= 8 {
			bits := binary.BigEndian.Uint64(cb.Payload[0:8])
			t := math.Float64frombits(bits)
			fedAmb.TimeAdvanceGrant(t)
		}
	}
}
