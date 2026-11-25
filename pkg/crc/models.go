package crc

import (
	"fmt"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// ProtocolType identifica o dialeto de transporte e serialização usado pelo federado
type ProtocolType = common.ProtocolType

const (
	ProtocolNative  = common.ProtocolNative
	ProtocolPortico = common.ProtocolPortico
)

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

	omt          *core.OMTRegistry
	router       *core.InterestMatcher
	stateTree    *core.FederationStateTree
	timeMgr      *core.TimeManager
	ownershipMgr *core.OwnershipManager
}

// NewFederationExecution cria uma nova federação com contadores e mapas inicializados
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
		nextFederateHandle: 1, // Handles começam em 1
		nextObjectHandle:   1,
		omt:                omt,
		router:             router,
		stateTree:          stateTree,
		timeMgr:            timeMgr,
		ownershipMgr:       ownershipMgr,
	}
}

// OMT retorna o registro de modelo de objetos (OMT) da federação
func (f *FederationExecution) OMT() *core.OMTRegistry {
	return f.omt
}

// Router retorna o roteador de pub/sub (InterestMatcher) da federação
func (f *FederationExecution) Router() *core.InterestMatcher {
	return f.router
}

// StateTree retorna a árvore de estado criptográfico Merkle da federação
func (f *FederationExecution) StateTree() *core.FederationStateTree {
	return f.stateTree
}

// TimeManager retorna o coordenador de avanço temporal determinístico da federação
func (f *FederationExecution) TimeManager() *core.TimeManager {
	return f.timeMgr
}

// OwnershipManager retorna o gerenciador de posse de atributos da federação
func (f *FederationExecution) OwnershipManager() *core.OwnershipManager {
	return f.ownershipMgr
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

	if f.timeMgr != nil {
		f.timeMgr.RegisterFederate(common.FederateHandle(id))
	}

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

	// Limpar interesses e publicações do federado no roteador
	if f.router != nil {
		f.router.RemoveFederate(common.FederateHandle(id))
	}

	// Remover federado do motor temporal
	if f.timeMgr != nil {
		f.timeMgr.UnregisterFederate(common.FederateHandle(id))
	}

	// Limpar posse de atributos do federado
	if f.ownershipMgr != nil {
		f.ownershipMgr.RemoveFederate(common.FederateHandle(id))
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
	if f.stateTree != nil {
		f.stateTree.UpdateObjectState(common.ObjectInstanceHandle(handle), obj.MerkleRoot)
	}

	// Atribuir posse inicial de atributos ao registrador
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

// UpdateObjectAttributes atualiza os atributos de uma instância e recalcula a raiz Merkle
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

	// Se a raiz Merkle não foi fornecida previamente, computar deterministicamente via SSZ
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

// DeleteObjectInstance remove a entidade da simulação validando posse ou privilégio de exclusão
func (f *FederationExecution) DeleteObjectInstance(objHandle int32, federateID int32, tag []byte) (*ObjectInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	obj, exists := f.objects[objHandle]
	if !exists {
		return nil, common.ErrObjectInstanceNotKnown
	}

	// Validação estrita da norma IEEE 1516-2025:
	// Apenas o proprietário que registrou a entidade OU quem detém privilégio de exclusão pode deletar
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

// GetAllObjects retorna um snapshot thread-safe de todas as instâncias de objetos ativas
func (f *FederationExecution) GetAllObjects() []*ObjectInstance {
	f.mu.RLock()
	defer f.mu.RUnlock()

	list := make([]*ObjectInstance, 0, len(f.objects))
	for _, obj := range f.objects {
		list = append(list, obj)
	}
	return list
}

// GetObjectInstance busca uma instância de objeto pelo handle
func (f *FederationExecution) GetObjectInstance(handle int32) (*ObjectInstance, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	obj, exists := f.objects[handle]
	return obj, exists
}

