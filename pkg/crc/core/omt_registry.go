package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

var (
	ErrClassNotFound        = common.ErrClassNotFound
	ErrAttributeNotFound    = common.ErrAttributeNotFound
	ErrInteractionNotFound  = common.ErrInteractionNotFound
	ErrParameterNotFound    = common.ErrParameterNotFound
	ErrDuplicateClass       = common.ErrDuplicateClass
	ErrDuplicateAttribute   = common.ErrDuplicateAttribute
	ErrDuplicateInteraction = common.ErrDuplicateInteraction
	ErrDuplicateParameter   = common.ErrDuplicateParameter
	ErrInvalidParentClass   = common.ErrInvalidParentClass
	ErrAmbiguousClass       = common.ErrAmbiguousClass
	ErrAmbiguousInteraction = common.ErrAmbiguousInteraction
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

// DimensionDef defines metadata for a DDM routing space dimension
type DimensionDef struct {
	Handle     common.DimensionHandle
	Name       string
	Datatype   string
	UpperBound int64
}

// ObjectClass defines an object class in the HLA OMT hierarchy
type ObjectClass struct {
	Handle        common.ObjectClassHandle
	Name          string // Simple name, e.g. "Sensor"
	QualifiedName string // Canonical FQCN, e.g. "HLAobjectRoot.Platform.Sensor"
	ParentHandle  common.ObjectClassHandle
	Children      []common.ObjectClassHandle

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
	Handle            common.InteractionClassHandle
	Name              string // Simple name, e.g. "Scan"
	QualifiedName     string // Canonical FQCN, e.g. "HLAinteractionRoot.Radar.Scan"
	ParentHandle      common.InteractionClassHandle
	Children          []common.InteractionClassHandle

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
	objectClasses             map[common.ObjectClassHandle]*ObjectClass
	objectFQCNToClass         map[string]common.ObjectClassHandle
	objectSimpleNameToHandles map[string][]common.ObjectClassHandle

	// Interaction Classes
	interactionClasses             map[common.InteractionClassHandle]*InteractionClass
	interactionFQCNToClass         map[string]common.InteractionClassHandle
	interactionSimpleNameToHandles map[string][]common.InteractionClassHandle

	// DDM Dimensions
	dimensions            map[common.DimensionHandle]*DimensionDef
	dimensionNameToHandle map[string]common.DimensionHandle

	// Monotonic Sequential Handle Generators
	nextObjectClassHandle      common.ObjectClassHandle
	nextAttributeHandle        common.AttributeHandle
	nextInteractionClassHandle common.InteractionClassHandle
	nextParameterHandle        common.ParameterHandle
	nextDimensionHandle        common.DimensionHandle
}

// NewOMTRegistry initializes the OMT registry, pre-populating native Standard MIM
// (HLAobjectRoot with HLAprivilegeToDeleteObject, and HLAinteractionRoot)
func NewOMTRegistry() *OMTRegistry {
	reg := &OMTRegistry{
		objectClasses:                  make(map[common.ObjectClassHandle]*ObjectClass),
		objectFQCNToClass:              make(map[string]common.ObjectClassHandle),
		objectSimpleNameToHandles:      make(map[string][]common.ObjectClassHandle),
		interactionClasses:             make(map[common.InteractionClassHandle]*InteractionClass),
		interactionFQCNToClass:         make(map[string]common.InteractionClassHandle),
		interactionSimpleNameToHandles: make(map[string][]common.InteractionClassHandle),
		dimensions:                     make(map[common.DimensionHandle]*DimensionDef),
		dimensionNameToHandle:          make(map[string]common.DimensionHandle),
		nextObjectClassHandle:          1001,
		nextAttributeHandle:            1001,
		nextInteractionClassHandle:     1001,
		nextParameterHandle:            1001,
		nextDimensionHandle:            1001,
	}

	// 1. Install HLAobjectRoot with HLAprivilegeToDeleteObject
	objRoot := &ObjectClass{
		Handle:           HLAobjectRootHandle,
		Name:             HLAobjectRootName,
		QualifiedName:    HLAobjectRootName,
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
	reg.objectFQCNToClass[HLAobjectRootName] = HLAobjectRootHandle
	reg.objectSimpleNameToHandles[HLAobjectRootName] = []common.ObjectClassHandle{HLAobjectRootHandle}

	// 2. Install HLAinteractionRoot
	intRoot := &InteractionClass{
		Handle:            HLAinteractionRootHandle,
		Name:              HLAinteractionRootName,
		QualifiedName:     HLAinteractionRootName,
		ParentHandle:      0,
		Children:          make([]common.InteractionClassHandle, 0),
		Parameters:        make(map[common.ParameterHandle]*ParameterDef),
		ParamNameToHandle: make(map[string]common.ParameterHandle),
		Ancestors:         nil,
	}
	reg.interactionClasses[HLAinteractionRootHandle] = intRoot
	reg.interactionFQCNToClass[HLAinteractionRootName] = HLAinteractionRootHandle
	reg.interactionSimpleNameToHandles[HLAinteractionRootName] = []common.InteractionClassHandle{HLAinteractionRootHandle}

	// Compute initial Euler intervals
	reg.recomputeIntervalsLocked()

	return reg
}

// AddObjectClass registers a new object subclass in the OMT hierarchy
func (r *OMTRegistry) AddObjectClass(name string, parentHandle common.ObjectClassHandle) (*ObjectClass, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("hla omt: empty object class name")
	}

	parent, exists := r.objectClasses[parentHandle]
	if !exists {
		return nil, ErrInvalidParentClass
	}

	simpleName := name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		simpleName = name[idx+1:]
	}

	var fqcn string
	if parent.QualifiedName == "" || parent.QualifiedName == HLAobjectRootName {
		fqcn = HLAobjectRootName + "." + simpleName
	} else {
		fqcn = parent.QualifiedName + "." + simpleName
	}

	if _, exists := r.objectFQCNToClass[fqcn]; exists {
		return nil, ErrDuplicateClass
	}

	handle := r.nextObjectClassHandle
	r.nextObjectClassHandle++

	ancestors := make([]common.ObjectClassHandle, 0, len(parent.Ancestors)+1)
	ancestors = append(ancestors, parentHandle)
	ancestors = append(ancestors, parent.Ancestors...)

	class := &ObjectClass{
		Handle:           handle,
		Name:             simpleName,
		QualifiedName:    fqcn,
		ParentHandle:     parentHandle,
		Children:         make([]common.ObjectClassHandle, 0),
		Attributes:       make(map[common.AttributeHandle]*AttributeDef),
		AttrNameToHandle: make(map[string]common.AttributeHandle),
		Ancestors:        ancestors,
	}

	parent.Children = append(parent.Children, handle)
	r.objectClasses[handle] = class
	r.objectFQCNToClass[fqcn] = handle
	r.objectSimpleNameToHandles[simpleName] = append(r.objectSimpleNameToHandles[simpleName], handle)

	r.recomputeIntervalsLocked()
	return class, nil
}

// AddInteractionClass registers a new interaction subclass in the OMT hierarchy
func (r *OMTRegistry) AddInteractionClass(name string, parentHandle common.InteractionClassHandle) (*InteractionClass, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("hla omt: empty interaction class name")
	}

	parent, exists := r.interactionClasses[parentHandle]
	if !exists {
		return nil, ErrInvalidParentClass
	}

	simpleName := name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		simpleName = name[idx+1:]
	}

	var fqcn string
	if parent.QualifiedName == "" || parent.QualifiedName == HLAinteractionRootName {
		fqcn = HLAinteractionRootName + "." + simpleName
	} else {
		fqcn = parent.QualifiedName + "." + simpleName
	}

	if _, exists := r.interactionFQCNToClass[fqcn]; exists {
		return nil, ErrDuplicateInteraction
	}

	handle := r.nextInteractionClassHandle
	r.nextInteractionClassHandle++

	ancestors := make([]common.InteractionClassHandle, 0, len(parent.Ancestors)+1)
	ancestors = append(ancestors, parentHandle)
	ancestors = append(ancestors, parent.Ancestors...)

	interaction := &InteractionClass{
		Handle:            handle,
		Name:              simpleName,
		QualifiedName:     fqcn,
		ParentHandle:      parentHandle,
		Children:          make([]common.InteractionClassHandle, 0),
		Parameters:        make(map[common.ParameterHandle]*ParameterDef),
		ParamNameToHandle: make(map[string]common.ParameterHandle),
		Ancestors:         ancestors,
	}

	parent.Children = append(parent.Children, handle)
	r.interactionClasses[handle] = interaction
	r.interactionFQCNToClass[fqcn] = handle
	r.interactionSimpleNameToHandles[simpleName] = append(r.interactionSimpleNameToHandles[simpleName], handle)

	r.recomputeIntervalsLocked()
	return interaction, nil
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

// GetChildObjectClass retrieves a direct child object class of parent by simple name
func (r *OMTRegistry) GetChildObjectClass(parentHandle common.ObjectClassHandle, name string) (*ObjectClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	parent, exists := r.objectClasses[parentHandle]
	if !exists {
		return nil, false
	}

	simpleName := name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		simpleName = name[idx+1:]
	}

	for _, childHandle := range parent.Children {
		child := r.objectClasses[childHandle]
		if child != nil && child.Name == simpleName {
			return child, true
		}
	}
	return nil, false
}

// GetChildInteractionClass retrieves a direct child interaction class of parent by simple name
func (r *OMTRegistry) GetChildInteractionClass(parentHandle common.InteractionClassHandle, name string) (*InteractionClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	parent, exists := r.interactionClasses[parentHandle]
	if !exists {
		return nil, false
	}

	simpleName := name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		simpleName = name[idx+1:]
	}

	for _, childHandle := range parent.Children {
		child := r.interactionClasses[childHandle]
		if child != nil && child.Name == simpleName {
			return child, true
		}
	}
	return nil, false
}

// GetObjectClassByFQCN retrieves object class metadata by exact canonical FQCN.
func (r *OMTRegistry) GetObjectClassByFQCN(fqcn string) (*ObjectClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.objectFQCNToClass[fqcn]
	if !ok {
		return nil, false
	}
	return r.objectClasses[h], true
}

// GetInteractionClassByFQCN retrieves interaction class metadata by exact canonical FQCN.
func (r *OMTRegistry) GetInteractionClassByFQCN(fqcn string) (*InteractionClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.interactionFQCNToClass[fqcn]
	if !ok {
		return nil, false
	}
	return r.interactionClasses[h], true
}

// ResolveObjectClass resolves an object class by FQCN, relative qualified name, or unambiguous simple name.
func (r *OMTRegistry) ResolveObjectClass(name string) (*ObjectClass, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrClassNotFound
	}

	// 1. Exact FQCN match
	if h, ok := r.objectFQCNToClass[name]; ok {
		return r.objectClasses[h], nil
	}

	// 2. Prepend HLAobjectRoot if missing
	if !strings.HasPrefix(name, HLAobjectRootName+".") && name != HLAobjectRootName {
		prefixed := HLAobjectRootName + "." + name
		if h, ok := r.objectFQCNToClass[prefixed]; ok {
			return r.objectClasses[h], nil
		}
	}

	// 3. Simple name match (if no dots)
	if !strings.Contains(name, ".") {
		handles := r.objectSimpleNameToHandles[name]
		if len(handles) == 1 {
			return r.objectClasses[handles[0]], nil
		}
		if len(handles) > 1 {
			return nil, ErrAmbiguousClass
		}
	}

	// 4. Suffix match (e.g. "Platform.Sensor")
	suffix := "." + name
	var matchedHandle common.ObjectClassHandle
	matchCount := 0
	for fqcn, handle := range r.objectFQCNToClass {
		if strings.HasSuffix(fqcn, suffix) {
			matchedHandle = handle
			matchCount++
		}
	}
	if matchCount == 1 {
		return r.objectClasses[matchedHandle], nil
	}
	if matchCount > 1 {
		return nil, ErrAmbiguousClass
	}

	return nil, ErrClassNotFound
}

// ResolveInteractionClass resolves an interaction class by FQCN, relative qualified name, or unambiguous simple name.
func (r *OMTRegistry) ResolveInteractionClass(name string) (*InteractionClass, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInteractionNotFound
	}

	// 1. Exact FQCN match
	if h, ok := r.interactionFQCNToClass[name]; ok {
		return r.interactionClasses[h], nil
	}

	// 2. Prepend HLAinteractionRoot if missing
	if !strings.HasPrefix(name, HLAinteractionRootName+".") && name != HLAinteractionRootName {
		prefixed := HLAinteractionRootName + "." + name
		if h, ok := r.interactionFQCNToClass[prefixed]; ok {
			return r.interactionClasses[h], nil
		}
	}

	// 3. Simple name match (if no dots)
	if !strings.Contains(name, ".") {
		handles := r.interactionSimpleNameToHandles[name]
		if len(handles) == 1 {
			return r.interactionClasses[handles[0]], nil
		}
		if len(handles) > 1 {
			return nil, ErrAmbiguousInteraction
		}
	}

	// 4. Suffix match
	suffix := "." + name
	var matchedHandle common.InteractionClassHandle
	matchCount := 0
	for fqcn, handle := range r.interactionFQCNToClass {
		if strings.HasSuffix(fqcn, suffix) {
			matchedHandle = handle
			matchCount++
		}
	}
	if matchCount == 1 {
		return r.interactionClasses[matchedHandle], nil
	}
	if matchCount > 1 {
		return nil, ErrAmbiguousInteraction
	}

	return nil, ErrInteractionNotFound
}

// GetObjectClassByName retrieves object class metadata by FQCN, relative qualified name, or unambiguous simple name.
func (r *OMTRegistry) GetObjectClassByName(name string) (*ObjectClass, bool) {
	cls, err := r.ResolveObjectClass(name)
	if err != nil {
		return nil, false
	}
	return cls, true
}

// GetInteractionClass retrieves interaction class metadata by handle
func (r *OMTRegistry) GetInteractionClass(handle common.InteractionClassHandle) (*InteractionClass, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, exists := r.interactionClasses[handle]
	return i, exists
}

// GetInteractionClassByName retrieves interaction class metadata by FQCN, relative qualified name, or unambiguous simple name.
func (r *OMTRegistry) GetInteractionClassByName(name string) (*InteractionClass, bool) {
	inter, err := r.ResolveInteractionClass(name)
	if err != nil {
		return nil, false
	}
	return inter, true
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

// RegisterDimension registers a DDM dimension into the OMT
func (r *OMTRegistry) RegisterDimension(name string, datatype string, upperBound int64) (common.DimensionHandle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if h, exists := r.dimensionNameToHandle[name]; exists {
		return h, nil
	}

	h := r.nextDimensionHandle
	r.nextDimensionHandle++

	dim := &DimensionDef{
		Handle:     h,
		Name:       name,
		Datatype:   datatype,
		UpperBound: upperBound,
	}
	r.dimensions[h] = dim
	r.dimensionNameToHandle[name] = h
	return h, nil
}

// GetDimension returns a dimension definition by handle
func (r *OMTRegistry) GetDimension(handle common.DimensionHandle) (*DimensionDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dim, ok := r.dimensions[handle]
	return dim, ok
}

// GetDimensionByName returns a dimension definition by name
func (r *OMTRegistry) GetDimensionByName(name string) (*DimensionDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.dimensionNameToHandle[name]
	if !ok {
		return nil, false
	}
	dim, ok := r.dimensions[h]
	return dim, ok
}

// GetAllDimensions returns all registered dimensions
func (r *OMTRegistry) GetAllDimensions() []*DimensionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dims := make([]*DimensionDef, 0, len(r.dimensions))
	for _, d := range r.dimensions {
		dims = append(dims, d)
	}
	return dims
}

// EnsureObjectClass implements metamodel.OMTBuilder, returning an existing child class or creating it.
func (r *OMTRegistry) EnsureObjectClass(parentHandle common.ObjectClassHandle, name string) (common.ObjectClassHandle, error) {
	if existing, exists := r.GetChildObjectClass(parentHandle, name); exists {
		return existing.Handle, nil
	}
	cls, err := r.AddObjectClass(name, parentHandle)
	if err != nil {
		return 0, err
	}
	return cls.Handle, nil
}

// EnsureAttribute implements metamodel.OMTBuilder, returning an existing attribute or creating it.
func (r *OMTRegistry) EnsureAttribute(classHandle common.ObjectClassHandle, name, datatype string) (common.AttributeHandle, error) {
	if existing, err := r.ResolveAttributeByName(classHandle, name); err == nil {
		return existing.Handle, nil
	}
	attr, err := r.AddAttribute(classHandle, name, datatype)
	if err != nil {
		return 0, err
	}
	return attr.Handle, nil
}

// EnsureInteractionClass implements metamodel.OMTBuilder, returning an existing child interaction or creating it.
func (r *OMTRegistry) EnsureInteractionClass(parentHandle common.InteractionClassHandle, name string) (common.InteractionClassHandle, error) {
	if existing, exists := r.GetChildInteractionClass(parentHandle, name); exists {
		return existing.Handle, nil
	}
	inter, err := r.AddInteractionClass(name, parentHandle)
	if err != nil {
		return 0, err
	}
	return inter.Handle, nil
}

// EnsureParameter implements metamodel.OMTBuilder, returning an existing parameter or creating it.
func (r *OMTRegistry) EnsureParameter(interHandle common.InteractionClassHandle, name, datatype string) (common.ParameterHandle, error) {
	param, err := r.AddParameter(interHandle, name, datatype)
	if err == nil {
		return param.Handle, nil
	}
	if errors.Is(err, ErrDuplicateParameter) {
		r.mu.RLock()
		defer r.mu.RUnlock()
		if inter, ok := r.interactionClasses[interHandle]; ok {
			if h, exists := inter.ParamNameToHandle[name]; exists {
				return h, nil
			}
		}
	}
	return 0, err
}

// EnsureDimension implements metamodel.OMTBuilder, returning an existing dimension or registering it.
func (r *OMTRegistry) EnsureDimension(name, datatype string, upperBound int64) (common.DimensionHandle, error) {
	if existing, ok := r.GetDimensionByName(name); ok {
		return existing.Handle, nil
	}
	return r.RegisterDimension(name, datatype, upperBound)
}
