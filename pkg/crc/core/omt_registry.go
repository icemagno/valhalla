package core

import (
	"errors"
	"fmt"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

var (
	ErrClassNotFound        = errors.New("hla omt: class not found in FOM registry")
	ErrAttributeNotFound    = errors.New("hla omt: attribute not found in class hierarchy")
	ErrInteractionNotFound  = errors.New("hla omt: interaction class not found in FOM registry")
	ErrParameterNotFound    = errors.New("hla omt: parameter not found in interaction hierarchy")
	ErrDuplicateClass       = errors.New("hla omt: class name already registered")
	ErrDuplicateAttribute   = errors.New("hla omt: attribute name already exists in class")
	ErrDuplicateInteraction = errors.New("hla omt: interaction name already registered")
	ErrDuplicateParameter   = errors.New("hla omt: parameter name already exists in interaction")
	ErrInvalidParentClass   = errors.New("hla omt: invalid or non-existent parent class")
)

// Constantes normativas do Standard MIM (IEEE 1516)
const (
	HLAobjectRootName                = common.HLAobjectRootName
	HLAobjectRootHandle              = common.HLAobjectRootHandle
	HLAprivilegeToDeleteObjectName   = common.HLAprivilegeToDeleteObjectName
	HLAprivilegeToDeleteObjectHandle = common.HLAprivilegeToDeleteObjectHandle

	HLAinteractionRootName   = common.HLAinteractionRootName
	HLAinteractionRootHandle = common.HLAinteractionRootHandle
)

// AttributeDef define os metadados de um atributo em uma classe de objeto
type AttributeDef struct {
	Handle      common.AttributeHandle
	Name        string
	Datatype    string
	ClassHandle common.ObjectClassHandle
}

// ObjectClass define uma classe de objeto na árvore OMT do HLA
type ObjectClass struct {
	Handle       common.ObjectClassHandle
	Name         string
	ParentHandle common.ObjectClassHandle
	Children     []common.ObjectClassHandle

	// Atributos definidos diretamente nesta classe
	Attributes        map[common.AttributeHandle]*AttributeDef
	AttrNameToHandle  map[string]common.AttributeHandle

	// Interval Encoding (Euler Tour) para verificação polimórfica O(1)
	IntervalLeft  uint32
	IntervalRight uint32

	// Lista linear pré-computada de ancestrais (do pai imediato até HLAobjectRoot)
	Ancestors []common.ObjectClassHandle
}

// ParameterDef define os metadados de um parâmetro de interação
type ParameterDef struct {
	Handle            common.ParameterHandle
	Name              string
	Datatype          string
	InteractionHandle common.InteractionClassHandle
}

// InteractionClass define uma classe de interação na árvore OMT do HLA
type InteractionClass struct {
	Handle       common.InteractionClassHandle
	Name         string
	ParentHandle common.InteractionClassHandle
	Children     []common.InteractionClassHandle

	// Parâmetros definidos diretamente nesta classe
	Parameters        map[common.ParameterHandle]*ParameterDef
	ParamNameToHandle map[string]common.ParameterHandle

	// Interval Encoding (Euler Tour) para verificação polimórfica O(1)
	IntervalLeft  uint32
	IntervalRight uint32

	// Lista linear pré-computada de ancestrais (do pai imediato até HLAinteractionRoot)
	Ancestors []common.InteractionClassHandle
}

// OMTRegistry gerencia o metamodelo de classes e interações da federação.
// Pré-populado com o Standard MIM do IEEE 1516 e otimizado com Interval Encoding.
type OMTRegistry struct {
	mu sync.RWMutex

	// Classes de Objeto
	objectClasses     map[common.ObjectClassHandle]*ObjectClass
	objectNameToClass map[string]common.ObjectClassHandle

	// Classes de Interação
	interactionClasses     map[common.InteractionClassHandle]*InteractionClass
	interactionNameToClass map[string]common.InteractionClassHandle

	// Geradores de Handles sequenciais monotônicos
	nextObjectClassHandle      common.ObjectClassHandle
	nextAttributeHandle        common.AttributeHandle
	nextInteractionClassHandle common.InteractionClassHandle
	nextParameterHandle        common.ParameterHandle
}

// NewOMTRegistry inicializa o registro OMT carregando o Standard MIM nativo (HLAobjectRoot e HLAinteractionRoot)
func NewOMTRegistry() *OMTRegistry {
	reg := &OMTRegistry{
		objectClasses:              make(map[common.ObjectClassHandle]*ObjectClass),
		objectNameToClass:          make(map[string]common.ObjectClassHandle),
		interactionClasses:         make(map[common.InteractionClassHandle]*InteractionClass),
		interactionNameToClass:     make(map[string]common.InteractionClassHandle),
		nextObjectClassHandle:      2,
		nextAttributeHandle:        2,
		nextInteractionClassHandle: 2,
		nextParameterHandle:        1,
	}

	// 1. Instalar HLAobjectRoot com HLAprivilegeToDeleteObject
	objRoot := &ObjectClass{
		Handle:           HLAobjectRootHandle,
		Name:             HLAobjectRootName,
		ParentHandle:     0,
		Children:         make([]common.ObjectClassHandle, 0),
		Attributes:       make(map[common.AttributeHandle]*AttributeDef),
		AttrNameToHandle: make(map[string]common.AttributeHandle),
		Ancestors:        nil,
	}
	privAttr := &AttributeDef{
		Handle:      HLAprivilegeToDeleteObjectHandle,
		Name:        HLAprivilegeToDeleteObjectName,
		Datatype:    "HLAhandle",
		ClassHandle: HLAobjectRootHandle,
	}
	objRoot.Attributes[privAttr.Handle] = privAttr
	objRoot.AttrNameToHandle[privAttr.Name] = privAttr.Handle

	reg.objectClasses[HLAobjectRootHandle] = objRoot
	reg.objectNameToClass[HLAobjectRootName] = HLAobjectRootHandle

	// 2. Instalar HLAinteractionRoot
	intRoot := &InteractionClass{
		Handle:            HLAinteractionRootHandle,
		Name:              HLAinteractionRootName,
		ParentHandle:      0,
		Children:          make([]common.InteractionClassHandle, 0),
		Parameters:        make(map[common.ParameterHandle]*ParameterDef),
		ParamNameToHandle: make(map[string]common.ParameterHandle),
		Ancestors:         nil,
	}
	reg.interactionClasses[HLAinteractionRootHandle] = intRoot
	reg.interactionNameToClass[HLAinteractionRootName] = HLAinteractionRootHandle

	// Computar intervalos iniciais
	reg.recomputeIntervalsLocked()

	return reg
}

// AddObjectClass registra uma nova subclasse de objeto na árvore OMT
func (r *OMTRegistry) AddObjectClass(name string, parentHandle common.ObjectClassHandle) (*ObjectClass, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.objectNameToClass[name]; exists {
		return nil, ErrDuplicateClass
	}

	parent, exists := r.objectClasses[parentHandle]
	if !exists {
		return nil, ErrInvalidParentClass
	}

	handle := r.nextObjectClassHandle
	r.nextObjectClassHandle++

	ancestors := make([]common.ObjectClassHandle, 0, len(parent.Ancestors)+1)
	ancestors = append(ancestors, parentHandle)
	ancestors = append(ancestors, parent.Ancestors...)

	class := &ObjectClass{
		Handle:           handle,
		Name:             name,
		ParentHandle:     parentHandle,
		Children:         make([]common.ObjectClassHandle, 0),
		Attributes:       make(map[common.AttributeHandle]*AttributeDef),
		AttrNameToHandle: make(map[string]common.AttributeHandle),
		Ancestors:        ancestors,
	}

	parent.Children = append(parent.Children, handle)
	r.objectClasses[handle] = class
	r.objectNameToClass[name] = handle

	r.recomputeIntervalsLocked()
	return class, nil
}

// AddAttribute adiciona um novo atributo a uma classe de objeto existente
func (r *OMTRegistry) AddAttribute(classHandle common.ObjectClassHandle, name, datatype string) (*AttributeDef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	class, exists := r.objectClasses[classHandle]
	if !exists {
		return nil, ErrClassNotFound
	}

	if _, exists := class.AttrNameToHandle[name]; exists {
		return nil, ErrDuplicateAttribute
	}

	handle := r.nextAttributeHandle
	r.nextAttributeHandle++

	attr := &AttributeDef{
		Handle:      handle,
		Name:        name,
		Datatype:    datatype,
		ClassHandle: classHandle,
	}

	class.Attributes[handle] = attr
	class.AttrNameToHandle[name] = handle
	return attr, nil
}

// AddInteractionClass registra uma nova subclasse de interação na árvore OMT
func (r *OMTRegistry) AddInteractionClass(name string, parentHandle common.InteractionClassHandle) (*InteractionClass, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.interactionNameToClass[name]; exists {
		return nil, ErrDuplicateInteraction
	}

	parent, exists := r.interactionClasses[parentHandle]
	if !exists {
		return nil, ErrInvalidParentClass
	}

	handle := r.nextInteractionClassHandle
	r.nextInteractionClassHandle++

	ancestors := make([]common.InteractionClassHandle, 0, len(parent.Ancestors)+1)
	ancestors = append(ancestors, parentHandle)
	ancestors = append(ancestors, parent.Ancestors...)

	interaction := &InteractionClass{
		Handle:            handle,
		Name:              name,
		ParentHandle:      parentHandle,
		Children:          make([]common.InteractionClassHandle, 0),
		Parameters:        make(map[common.ParameterHandle]*ParameterDef),
		ParamNameToHandle: make(map[string]common.ParameterHandle),
		Ancestors:         ancestors,
	}

	parent.Children = append(parent.Children, handle)
	r.interactionClasses[handle] = interaction
	r.interactionNameToClass[name] = handle

	r.recomputeIntervalsLocked()
	return interaction, nil
}

// AddParameter adiciona um parâmetro a uma classe de interação existente
func (r *OMTRegistry) AddParameter(interactionHandle common.InteractionClassHandle, name, datatype string) (*ParameterDef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	interaction, exists := r.interactionClasses[interactionHandle]
	if !exists {
		return nil, ErrInteractionNotFound
	}

	if _, exists := interaction.ParamNameToHandle[name]; exists {
		return nil, ErrDuplicateParameter
	}

	handle := r.nextParameterHandle
	r.nextParameterHandle++

	param := &ParameterDef{
		Handle:            handle,
		Name:              name,
		Datatype:          datatype,
		InteractionHandle: interactionHandle,
	}

	interaction.Parameters[handle] = param
	interaction.ParamNameToHandle[name] = handle
	return param, nil
}

// recomputeIntervalsLocked executa uma travessia DFS Euler Tour para codificar intervalos [L, R]
func (r *OMTRegistry) recomputeIntervalsLocked() {
	// 1. Árvore de Objetos
	counter := uint32(1)
	r.eulerTourObjects(HLAobjectRootHandle, &counter)

	// 2. Árvore de Interações
	counter = uint32(1)
	r.eulerTourInteractions(HLAinteractionRootHandle, &counter)
}

func (r *OMTRegistry) eulerTourObjects(handle common.ObjectClassHandle, counter *uint32) {
	class, exists := r.objectClasses[handle]
	if !exists {
		return
	}
	class.IntervalLeft = *counter
	*counter++

	for _, childHandle := range class.Children {
		r.eulerTourObjects(childHandle, counter)
	}

	class.IntervalRight = *counter
	*counter++
}

func (r *OMTRegistry) eulerTourInteractions(handle common.InteractionClassHandle, counter *uint32) {
	interaction, exists := r.interactionClasses[handle]
	if !exists {
		return
	}
	interaction.IntervalLeft = *counter
	*counter++

	for _, childHandle := range interaction.Children {
		r.eulerTourInteractions(childHandle, counter)
	}

	interaction.IntervalRight = *counter
	*counter++
}

// IsSubclassOfObject determina em O(1) se child é a mesma classe ou subclasse de ancestor
func (r *OMTRegistry) IsSubclassOfObject(child, ancestor common.ObjectClassHandle) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, cExists := r.objectClasses[child]
	a, aExists := r.objectClasses[ancestor]
	if !cExists || !aExists {
		return false
	}

	return a.IntervalLeft <= c.IntervalLeft && c.IntervalRight <= a.IntervalRight
}

// IsSubclassOfInteraction determina em O(1) se child é a mesma interação ou subclasse de ancestor
func (r *OMTRegistry) IsSubclassOfInteraction(child, ancestor common.InteractionClassHandle) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, cExists := r.interactionClasses[child]
	a, aExists := r.interactionClasses[ancestor]
	if !cExists || !aExists {
		return false
	}

	return a.IntervalLeft <= c.IntervalLeft && c.IntervalRight <= a.IntervalRight
}

// GetObjectClass busca os metadados de uma classe pelo handle
func (r *OMTRegistry) GetObjectClass(handle common.ObjectClassHandle) (*ObjectClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, exists := r.objectClasses[handle]
	return c, exists
}

// GetObjectClassByName busca uma classe pelo nome
func (r *OMTRegistry) GetObjectClassByName(name string) (*ObjectClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, exists := r.objectNameToClass[name]
	if !exists {
		return nil, false
	}
	return r.objectClasses[h], true
}

// GetInteractionClass busca os metadados de uma interação pelo handle
func (r *OMTRegistry) GetInteractionClass(handle common.InteractionClassHandle) (*InteractionClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, exists := r.interactionClasses[handle]
	return i, exists
}

// GetInteractionClassByName busca uma interação pelo nome
func (r *OMTRegistry) GetInteractionClassByName(name string) (*InteractionClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, exists := r.interactionNameToClass[name]
	if !exists {
		return nil, false
	}
	return r.interactionClasses[h], true
}

// ResolveAttribute busca um atributo na classe ou em qualquer um dos seus ancestrais (herança HLA)
func (r *OMTRegistry) ResolveAttribute(classHandle common.ObjectClassHandle, attrHandle common.AttributeHandle) (*AttributeDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	currHandle := classHandle
	for currHandle != 0 {
		class, exists := r.objectClasses[currHandle]
		if !exists {
			break
		}
		if attr, ok := class.Attributes[attrHandle]; ok {
			return attr, nil
		}
		currHandle = class.ParentHandle
	}

	return nil, ErrAttributeNotFound
}

// ResolveAttributeByName busca o handle de um atributo pelo nome, subindo pela cadeia de herança
func (r *OMTRegistry) ResolveAttributeByName(classHandle common.ObjectClassHandle, name string) (*AttributeDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	currHandle := classHandle
	for currHandle != 0 {
		class, exists := r.objectClasses[currHandle]
		if !exists {
			break
		}
		if handle, ok := class.AttrNameToHandle[name]; ok {
			return class.Attributes[handle], nil
		}
		currHandle = class.ParentHandle
	}

	return nil, fmt.Errorf("%w: '%s' na hierarquia da classe %d", ErrAttributeNotFound, name, classHandle)
}

// GetClassAncestors retorna a lista ordenada de ancestrais de uma classe até HLAobjectRoot
func (r *OMTRegistry) GetClassAncestors(classHandle common.ObjectClassHandle) []common.ObjectClassHandle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	class, exists := r.objectClasses[classHandle]
	if !exists {
		return nil
	}
	res := make([]common.ObjectClassHandle, len(class.Ancestors))
	copy(res, class.Ancestors)
	return res
}

// GetInteractionAncestors retorna a lista ordenada de ancestrais de uma interação até HLAinteractionRoot
func (r *OMTRegistry) GetInteractionAncestors(interactionHandle common.InteractionClassHandle) []common.InteractionClassHandle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	interaction, exists := r.interactionClasses[interactionHandle]
	if !exists {
		return nil
	}
	res := make([]common.InteractionClassHandle, len(interaction.Ancestors))
	copy(res, interaction.Ancestors)
	return res
}
