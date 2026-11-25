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

// Normative Standard MIM Constants (IEEE 1516-2025)
const (
	HLAobjectRootName                = common.HLAobjectRootName
	HLAobjectRootHandle              = common.HLAobjectRootHandle
	HLAprivilegeToDeleteObjectName   = common.HLAprivilegeToDeleteObjectName
	HLAprivilegeToDeleteObjectHandle = common.HLAprivilegeToDeleteObjectHandle

	HLAinteractionRootName   = common.HLAinteractionRootName
	HLAinteractionRootHandle = common.HLAinteractionRootHandle
)

// AttributeDef defines metadata for an attribute within an object class
type AttributeDef struct {
	Handle      common.AttributeHandle
	Name        string
	Datatype    string
	ClassHandle common.ObjectClassHandle
}

// ObjectClass defines an object class in the HLA OMT hierarchy
type ObjectClass struct {
	Handle       common.ObjectClassHandle
	Name         string
	ParentHandle common.ObjectClassHandle
	Children     []common.ObjectClassHandle

	// Attributes defined directly in this class
	Attributes       map[common.AttributeHandle]*AttributeDef
	AttrNameToHandle map[string]common.AttributeHandle

	// Euler Tour Interval Encoding for O(1) polymorphic checks
	IntervalLeft  uint32
	IntervalRight uint32

	// Precomputed linear ancestor list (from immediate parent to HLAobjectRoot)
	Ancestors []common.ObjectClassHandle
}

// ParameterDef defines metadata for a parameter within an interaction class
type ParameterDef struct {
	Handle            common.ParameterHandle
	Name              string
	Datatype          string
	InteractionHandle common.InteractionClassHandle
}

// InteractionClass defines an interaction class in the HLA OMT hierarchy
type InteractionClass struct {
	Handle       common.InteractionClassHandle
	Name         string
	ParentHandle common.InteractionClassHandle
	Children     []common.InteractionClassHandle

	// Parameters defined directly in this interaction
	Parameters        map[common.ParameterHandle]*ParameterDef
	ParamNameToHandle map[string]common.ParameterHandle

	// Euler Tour Interval Encoding for O(1) polymorphic checks
	IntervalLeft  uint32
	IntervalRight uint32

	// Precomputed linear ancestor list (from immediate parent to HLAinteractionRoot)
	Ancestors []common.InteractionClassHandle
}

// OMTRegistry manages the federation's class and interaction metamodel.
// Pre-populated with the IEEE 1516 Standard MIM and optimized with Euler Tour Interval Encoding.
type OMTRegistry struct {
	mu sync.RWMutex

	// Object Classes
	objectClasses     map[common.ObjectClassHandle]*ObjectClass
	objectNameToClass map[string]common.ObjectClassHandle

	// Interaction Classes
	interactionClasses     map[common.InteractionClassHandle]*InteractionClass
	interactionNameToClass map[string]common.InteractionClassHandle

	// Monotonic Sequential Handle Generators
	nextObjectClassHandle      common.ObjectClassHandle
	nextAttributeHandle        common.AttributeHandle
	nextInteractionClassHandle common.InteractionClassHandle
	nextParameterHandle        common.ParameterHandle
}

// NewOMTRegistry initializes the OMT registry, pre-populating native Standard MIM
// (HLAobjectRoot with HLAprivilegeToDeleteObject, and HLAinteractionRoot)
func NewOMTRegistry() *OMTRegistry {
	reg := &OMTRegistry{
		objectClasses:              make(map[common.ObjectClassHandle]*ObjectClass),
		objectNameToClass:          make(map[string]common.ObjectClassHandle),
		interactionClasses:         make(map[common.InteractionClassHandle]*InteractionClass),
		interactionNameToClass:     make(map[string]common.InteractionClassHandle),
		nextObjectClassHandle:      1001,
		nextAttributeHandle:        1001,
		nextInteractionClassHandle: 1001,
		nextParameterHandle:        1001,
	}

	// 1. Install HLAobjectRoot with HLAprivilegeToDeleteObject
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

	// 2. Install HLAinteractionRoot
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

	// Compute initial Euler intervals
	reg.recomputeIntervalsLocked()

	return reg
}

// AddObjectClass registers a new object subclass in the OMT hierarchy
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

// AddAttribute adds a new attribute to an existing object class
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

// AddInteractionClass registers a new interaction subclass in the OMT hierarchy
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

// AddParameter adds a parameter to an existing interaction class
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

// recomputeIntervalsLocked runs a DFS Euler Tour traversal to assign [L, R] intervals
func (r *OMTRegistry) recomputeIntervalsLocked() {
	// 1. Object Class Tree
	counter := uint32(1)
	r.eulerTourObjects(HLAobjectRootHandle, &counter)

	// 2. Interaction Class Tree
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

// IsSubclassOfObject determines in O(1) CPU time whether child is the same or a subclass of ancestor
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

// IsSubclassOfInteraction determines in O(1) CPU time whether child is the same or a subclass of ancestor
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

// GetObjectClass retrieves object class metadata by handle
func (r *OMTRegistry) GetObjectClass(handle common.ObjectClassHandle) (*ObjectClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, exists := r.objectClasses[handle]
	return c, exists
}

// GetObjectClassByName retrieves object class metadata by name
func (r *OMTRegistry) GetObjectClassByName(name string) (*ObjectClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, exists := r.objectNameToClass[name]
	if !exists {
		return nil, false
	}
	return r.objectClasses[h], true
}

// GetInteractionClass retrieves interaction class metadata by handle
func (r *OMTRegistry) GetInteractionClass(handle common.InteractionClassHandle) (*InteractionClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, exists := r.interactionClasses[handle]
	return i, exists
}

// GetInteractionClassByName retrieves interaction class metadata by name
func (r *OMTRegistry) GetInteractionClassByName(name string) (*InteractionClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, exists := r.interactionNameToClass[name]
	if !exists {
		return nil, false
	}
	return r.interactionClasses[h], true
}

// ResolveAttribute searches for an attribute in the class or its ancestors (HLA inheritance)
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

// ResolveAttributeByName searches for an attribute handle by name, ascending the inheritance chain
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

	return nil, fmt.Errorf("%w: '%s' in class %d hierarchy", ErrAttributeNotFound, name, classHandle)
}

// GetClassAncestors returns the ordered list of class ancestors up to HLAobjectRoot
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

// ForEachObjectClass iterates over all registered object classes under read lock
func (r *OMTRegistry) ForEachObjectClass(fn func(cls *ObjectClass)) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, cls := range r.objectClasses {
		fn(cls)
	}
}

// ForEachInteractionClass iterates over all registered interaction classes under read lock
func (r *OMTRegistry) ForEachInteractionClass(fn func(inter *InteractionClass)) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, inter := range r.interactionClasses {
		fn(inter)
	}
}

// GetInteractionAncestors returns the ordered list of interaction ancestors up to HLAinteractionRoot
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
