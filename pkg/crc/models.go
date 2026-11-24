package crc

import (
	"fmt"
	"sync"
	"time"
)

// ProtocolType identifica o dialeto de transporte e serialização usado pelo federado
type ProtocolType uint8

const (
	ProtocolNative  ProtocolType = 1 // Valhalla Nativo (SSZ / Merkle / PQC / VAL4)
	ProtocolPortico ProtocolType = 2 // Portico Legado (JOS / Header 12b / 0xCAFE / 0xBEEF)
)

func (p ProtocolType) String() string {
	switch p {
	case ProtocolNative:
		return "Valhalla-Native(VAL4)"
	case ProtocolPortico:
		return "Portico-Legacy(JOS)"
	default:
		return "Unknown"
	}
}

// Session define a interface de transporte para despacho assíncrono de mensagens a um federado
type Session interface {
	FederateID() int32
	Protocol() ProtocolType
	SendNotification(msgType uint8, payload []byte) error
	Close() error
}

// Federate representa um nó participante de uma execução de federação no CRC
type Federate struct {
	ID           int32
	Name         string
	FederateType string
	Protocol     ProtocolType
	Session      Session
	JoinedAt     time.Time

	// Estado de Gerenciamento Temporal (Time Management)
	CurrentTime   float64
	Lookahead     float64
	IsRegulating  bool
	IsConstrained bool
	IsAdvancing   bool
	TargetTime    float64
}

// SyncPoint representa um ponto de sincronização ativo na federação
type SyncPoint struct {
	Label             string
	Tag               []byte
	RegistrarID       int32
	AchievedFederates map[int32]bool
	IsSynchronized    bool
	CreatedAt         time.Time
}

// NewSyncPoint instancia um novo ponto de sincronização
func NewSyncPoint(label string, tag []byte, registrarID int32) *SyncPoint {
	return &SyncPoint{
		Label:             label,
		Tag:               tag,
		RegistrarID:       registrarID,
		AchievedFederates: make(map[int32]bool),
		CreatedAt:         time.Now(),
	}
}

// ObjectInstance representa uma instância viva de objeto registrada na simulação
type ObjectInstance struct {
	Handle      int32
	ClassHandle int32
	Name        string
	OwnerID     int32
	Attributes  map[int32][]byte
	MerkleRoot  [32]byte
	UpdatedAt   time.Time
}

// FederationExecution representa o estado de uma federação em execução no CRC
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
}

// NewFederationExecution cria uma nova federação com contadores e mapas inicializados
func NewFederationExecution(handle int32, name string) *FederationExecution {
	return &FederationExecution{
		Handle:             handle,
		Name:               name,
		federates:          make(map[int32]*Federate),
		nameToID:           make(map[string]int32),
		syncPoints:         make(map[string]*SyncPoint),
		objects:            make(map[int32]*ObjectInstance),
		nameToObjID:        make(map[string]int32),
		nextFederateHandle: 1, // Handles começam em 1
		nextObjectHandle:   1,
	}
}

// Join registra um federado na federação e atribui seu handle sequencial exclusivo
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

	return fed, nil
}

// Resign remove o federado da federação liberando seu nome e sessao
func (f *FederationExecution) Resign(id int32) (*Federate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	fed, exists := f.federates[id]
	if !exists {
		return nil, ErrFederateNotFound
	}

	delete(f.federates, id)
	delete(f.nameToID, fed.Name)

	// Remover de pontos de sincronização pendentes
	for _, sp := range f.syncPoints {
		delete(sp.AchievedFederates, id)
	}

	return fed, nil
}

// GetFederate obtém um federado pelo ID com thread-safety
func (f *FederationExecution) GetFederate(id int32) (*Federate, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	fed, exists := f.federates[id]
	return fed, exists
}

// RegisterSyncPoint registra um novo ponto de sincronização se o rótulo for único
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

// AchieveSyncPoint marca um ponto como conquistado por um federado e verifica sincronização global
func (f *FederationExecution) AchieveSyncPoint(label string, federateID int32) (isFederationSynchronized bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sp, exists := f.syncPoints[label]
	if !exists {
		return false, ErrSyncPointNotFound
	}

	sp.AchievedFederates[federateID] = true

	// Uma federação está sincronizada quando TODOS os federados atingiram o ponto
	if len(f.federates) > 0 && len(sp.AchievedFederates) >= len(f.federates) {
		sp.IsSynchronized = true
		return true, nil
	}

	return false, nil
}

// GetAllFederates retorna uma cópia de leitura de todos os federados ativos
func (f *FederationExecution) GetAllFederates() []*Federate {
	f.mu.RLock()
	defer f.mu.RUnlock()

	list := make([]*Federate, 0, len(f.federates))
	for _, fed := range f.federates {
		list = append(list, fed)
	}
	return list
}

// RegisterObjectInstance registra uma nova instância de objeto na federação
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
	return obj, nil
}

// UpdateObjectAttributes atualiza os atributos de uma instância com Merkle Root
func (f *FederationExecution) UpdateObjectAttributes(objHandle int32, attrs map[int32][]byte, merkleRoot [32]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	obj, exists := f.objects[objHandle]
	if !exists {
		return ErrObjectInstanceNotFound
	}

	for k, v := range attrs {
		obj.Attributes[k] = v
	}
	obj.MerkleRoot = merkleRoot
	obj.UpdatedAt = time.Now()
	return nil
}

// GetObjectInstance busca uma instância de objeto pelo handle
func (f *FederationExecution) GetObjectInstance(handle int32) (*ObjectInstance, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	obj, exists := f.objects[handle]
	return obj, exists
}

