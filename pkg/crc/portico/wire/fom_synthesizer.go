package wire

import (
	"sort"
	"strings"

	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

// SynthesizePorticoObjectModel creates a fully compliant Portico org.portico.lrc.model.ObjectModel
// from Valhalla's sovereign core.OMTRegistry, ensuring legacy Portico clients can resolve
// handles in local memory without NullPointerExceptions.
func SynthesizePorticoObjectModel(omt *core.OMTRegistry) *metamodel.JavaObject {
	if omt == nil {
		return nil
	}

	model := metamodel.NewJavaObject(metamodel.ClassDescObjectModel)
	oclassesMap := metamodel.NewJavaHashMap(make(map[any]any))
	iclassesMap := metamodel.NewJavaHashMap(make(map[any]any))
	datatypesMap := metamodel.NewJavaHashMap(make(map[any]any))
	spacesMap := metamodel.NewJavaHashMap(make(map[any]any))

	ocEntries := oclassesMap.Get("entries").(map[any]any)
	icEntries := iclassesMap.Get("entries").(map[any]any)

	model.Set("version", metamodel.EnumHLAVersion("IEEE1516e"))
	model.Set("locked", false)
	model.Set("filename", "")
	model.Set("oclasses", oclassesMap)
	model.Set("iclasses", iclassesMap)
	model.Set("datatypes", datatypesMap)
	model.Set("spaces", spacesMap)

	var maxHandle int32 = 100
	var privToDeleteHandle int32 = 0
	var ocRootObj *metamodel.JavaObject
	var icRootObj *metamodel.JavaObject

	// 1. First pass: instantiate all OCMetadata
	ocMap := make(map[uint32]*metamodel.JavaObject)
	var allObjects []*core.ObjectClass
	omt.ForEachObjectClass(func(cls *core.ObjectClass) {
		allObjects = append(allObjects, cls)
	})
	sort.Slice(allObjects, func(i, j int) bool { return allObjects[i].Handle < allObjects[j].Handle })

	for _, cls := range allObjects {
		h := int32(cls.Handle)
		if h > maxHandle {
			maxHandle = h
		}

		oc := metamodel.NewJavaObject(metamodel.ClassDescOCMetadata)
		oc.Set("handle", h)
		oc.Set("name", cls.Name)
		oc.Set("qualifiedName", cls.Name)
		oc.Set("vsafeQualifiedName", "")
		oc.Set("model", model)
		oc.Set("children", metamodel.NewJavaHashSet(nil))
		oc.Set("attributes", metamodel.NewJavaHashMap(make(map[any]any)))
		oc.Set("sharing", metamodel.EnumSharing("NEITHER"))

		ocMap[uint32(cls.Handle)] = oc
		ocEntries[metamodel.NewJavaInteger(h)] = oc

		if cls.Name == "HLAobjectRoot" {
			ocRootObj = oc
		}
	}

	// 2. Second pass: link parents, children, and attributes
	for _, cls := range allObjects {
		oc := ocMap[uint32(cls.Handle)]
		if cls.ParentHandle != 0 {
			if parentOC, exists := ocMap[uint32(cls.ParentHandle)]; exists {
				oc.Set("parent", parentOC)
				parentChildren := parentOC.Get("children").(*metamodel.JavaObject)
				// Add child to parent's children set
				parentChildren.Set("elements", append(parentChildren.Get("elements").([]any), oc))
			}
		}

		// Instantiate ACMetadata attributes
		attrsMap := oc.Get("attributes").(*metamodel.JavaObject)
		attrEntries := attrsMap.Get("entries").(map[any]any)

		for _, attr := range cls.Attributes {
			ah := int32(attr.Handle)
			if ah > maxHandle {
				maxHandle = ah
			}

			ac := metamodel.NewJavaObject(metamodel.ClassDescACMetadata)
			ac.Set("handle", ah)
			ac.Set("name", attr.Name)
			ac.Set("container", oc)
			ac.Set("order", metamodel.EnumOrder("TIMESTAMP"))
			ac.Set("sharing", metamodel.EnumSharing("NEITHER"))
			ac.Set("transport", metamodel.EnumTransport("RELIABLE"))
			ac.Set("space", nil)
			ac.Set("datatype", nil)

			attrEntries[metamodel.NewJavaInteger(ah)] = ac

			if strings.EqualFold(attr.Name, "HLAprivilegeToDeleteObject") {
				privToDeleteHandle = ah
			}
		}
	}

	// Compute true qualified names and version-safe qualified names by walking up the parent chain
	for _, oc := range ocMap {
		oc.Set("qualifiedName", getQualifiedName(oc))
		oc.Set("vsafeQualifiedName", getVersionSafeQualifiedName(oc))
	}

	// 3. First pass: instantiate all ICMetadata
	icMap := make(map[uint32]*metamodel.JavaObject)
	var allInteractions []*core.InteractionClass
	omt.ForEachInteractionClass(func(inter *core.InteractionClass) {
		allInteractions = append(allInteractions, inter)
	})
	sort.Slice(allInteractions, func(i, j int) bool { return allInteractions[i].Handle < allInteractions[j].Handle })

	for _, inter := range allInteractions {
		h := int32(inter.Handle)
		if h > maxHandle {
			maxHandle = h
		}

		ic := metamodel.NewJavaObject(metamodel.ClassDescICMetadata)
		ic.Set("handle", h)
		ic.Set("name", inter.Name)
		ic.Set("qualifiedName", inter.Name)
		ic.Set("vsafeQualifiedName", "")
		ic.Set("model", model)
		ic.Set("children", metamodel.NewJavaHashSet(nil))
		ic.Set("parameters", metamodel.NewJavaHashMap(make(map[any]any)))
		ic.Set("order", metamodel.EnumOrder("TIMESTAMP"))
		ic.Set("sharing", metamodel.EnumSharing("NEITHER"))
		ic.Set("transport", metamodel.EnumTransport("RELIABLE"))
		ic.Set("space", nil)

		icMap[uint32(inter.Handle)] = ic
		icEntries[metamodel.NewJavaInteger(h)] = ic

		if inter.Name == "HLAinteractionRoot" {
			icRootObj = ic
		}
	}

	// 4. Second pass: link parents, children, and parameters for interactions
	for _, inter := range allInteractions {
		ic := icMap[uint32(inter.Handle)]
		if inter.ParentHandle != 0 {
			if parentIC, exists := icMap[uint32(inter.ParentHandle)]; exists {
				ic.Set("parent", parentIC)
				parentChildren := parentIC.Get("children").(*metamodel.JavaObject)
				parentChildren.Set("elements", append(parentChildren.Get("elements").([]any), ic))
			}
		}

		// Instantiate PCMetadata parameters
		paramsMap := ic.Get("parameters").(*metamodel.JavaObject)
		paramEntries := paramsMap.Get("entries").(map[any]any)

		for _, param := range inter.Parameters {
			ph := int32(param.Handle)
			if ph > maxHandle {
				maxHandle = ph
			}

			pc := metamodel.NewJavaObject(metamodel.ClassDescPCMetadata)
			pc.Set("handle", ph)
			pc.Set("name", param.Name)
			pc.Set("container", ic)
			pc.Set("datatype", nil)

			paramEntries[metamodel.NewJavaInteger(ph)] = pc
		}
	}

	// Compute true qualified names and version-safe qualified names for interactions
	for _, ic := range icMap {
		ic.Set("qualifiedName", getQualifiedName(ic))
		ic.Set("vsafeQualifiedName", getVersionSafeQualifiedName(ic))
	}

	model.Set("ocroot", ocRootObj)
	model.Set("icroot", icRootObj)
	model.Set("privilegeToDelete", privToDeleteHandle)
	model.Set("handle", maxHandle+1)

	return model
}

func getQualifiedName(obj *metamodel.JavaObject) string {
	if obj == nil {
		return ""
	}
	name, _ := obj.Get("name").(string)
	parentAny := obj.Get("parent")
	if parentAny == nil {
		return name
	}
	parentObj, ok := parentAny.(*metamodel.JavaObject)
	if !ok || parentObj == nil {
		return name
	}
	parentQ := getQualifiedName(parentObj)
	if parentQ == "" {
		return name
	}
	return parentQ + "." + name
}

func getVersionSafeQualifiedName(obj *metamodel.JavaObject) string {
	if obj == nil {
		return ""
	}
	parentAny := obj.Get("parent")
	if parentAny == nil {
		return ""
	}
	parentObj, ok := parentAny.(*metamodel.JavaObject)
	if !ok || parentObj == nil {
		return ""
	}
	name, _ := obj.Get("name").(string)
	parentVSafe := getVersionSafeQualifiedName(parentObj)
	if parentVSafe == "" {
		return name
	}
	return parentVSafe + "." + name
}
