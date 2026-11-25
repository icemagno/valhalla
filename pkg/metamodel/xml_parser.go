package metamodel

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// XMLObjectModel represents the root <objectModel> element of an IEEE 1516-2010 FOM module.
type XMLObjectModel struct {
	XMLName      xml.Name              `xml:"objectModel"`
	Objects      []XMLObjectClass      `xml:"objects>objectClass"`
	Interactions []XMLInteractionClass `xml:"interactions>interactionClass"`
	Dimensions   []XMLDimension        `xml:"dimensions>dimension"`
}

// XMLDimension represents a <dimension> element in the FOM XML.
type XMLDimension struct {
	Name       string `xml:"name"`
	DataType   string `xml:"dataType"`
	UpperBound int64  `xml:"upperBound"`
}

// XMLObjectClass represents a hierarchical <objectClass> element in the FOM XML.
type XMLObjectClass struct {
	Name       string           `xml:"name"`
	Sharing    string           `xml:"sharing"`
	Attributes []XMLAttribute   `xml:"attribute"`
	SubClasses []XMLObjectClass `xml:"objectClass"`
}

// XMLAttribute represents an <attribute> element in the FOM XML.
type XMLAttribute struct {
	Name     string `xml:"name"`
	DataType string `xml:"dataType"`
}

// XMLInteractionClass represents a hierarchical <interactionClass> element in the FOM XML.
type XMLInteractionClass struct {
	Name       string                `xml:"name"`
	Sharing    string                `xml:"sharing"`
	Parameters []XMLParameter        `xml:"parameter"`
	SubClasses []XMLInteractionClass `xml:"interactionClass"`
}

// XMLParameter represents a <parameter> element in the FOM XML.
type XMLParameter struct {
	Name     string `xml:"name"`
	DataType string `xml:"dataType"`
}

// ParseFOMFile reads an IEEE 1516-2010 XML FOM file from disk and merges it into the OMTRegistry.
func ParseFOMFile(omt *core.OMTRegistry, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read FOM file '%s': %w", filePath, err)
	}
	return ParseFOMXML(omt, data)
}

// ParseFOMXML parses an in-memory byte slice containing an IEEE 1516-2010 XML FOM and merges
// its object classes, attributes, interactions, and parameters into the target OMTRegistry.
func ParseFOMXML(omt *core.OMTRegistry, data []byte) error {
	if omt == nil {
		return fmt.Errorf("target OMTRegistry cannot be nil")
	}

	var fom XMLObjectModel
	if err := xml.Unmarshal(data, &fom); err != nil {
		return fmt.Errorf("failed to parse FOM XML: %w", err)
	}

	// 1. Process Objects hierarchy
	for _, rootClass := range fom.Objects {
		if err := mergeObjectClass(omt, rootClass, 0); err != nil {
			return err
		}
	}

	// 2. Process Interactions hierarchy
	for _, rootInter := range fom.Interactions {
		if err := mergeInteractionClass(omt, rootInter, 0); err != nil {
			return err
		}
	}

	// 3. Process Dimensions
	for _, dim := range fom.Dimensions {
		name := strings.TrimSpace(dim.Name)
		if name != "" {
			_, _ = omt.RegisterDimension(name, dim.DataType, dim.UpperBound)
		}
	}

	return nil
}

func mergeObjectClass(omt *core.OMTRegistry, node XMLObjectClass, parentHandle common.ObjectClassHandle) error {
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return nil
	}

	var currentHandle common.ObjectClassHandle

	// Check if this class already exists (e.g. HLAobjectRoot or pre-registered in an earlier module)
	existing, exists := omt.GetObjectClassByName(name)
	if exists {
		currentHandle = existing.Handle
	} else {
		// If it's HLAobjectRoot, its handle is standard 1
		if name == core.HLAobjectRootName {
			currentHandle = core.HLAobjectRootHandle
		} else {
			// Ensure we have a valid parent
			if parentHandle == 0 {
				parentHandle = core.HLAobjectRootHandle
			}
			newCls, err := omt.AddObjectClass(name, parentHandle)
			if err != nil {
				return fmt.Errorf("failed to add object class '%s': %w", name, err)
			}
			currentHandle = newCls.Handle
		}
	}

	// Merge attributes defined on this class
	for _, attr := range node.Attributes {
		attrName := strings.TrimSpace(attr.Name)
		if attrName == "" {
			continue
		}
		// Skip privilegeToDelete if on HLAobjectRoot as it's already pre-installed
		if name == core.HLAobjectRootName && attrName == core.HLAprivilegeToDeleteObjectName {
			continue
		}
		// Only add if not already present
		if _, err := omt.ResolveAttributeByName(currentHandle, attrName); err != nil {
			dt := strings.TrimSpace(attr.DataType)
			if dt == "" {
				dt = "HLAASCIIstring"
			}
			if _, err := omt.AddAttribute(currentHandle, attrName, dt); err != nil {
				return fmt.Errorf("failed to add attribute '%s' to class '%s': %w", attrName, name, err)
			}
		}
	}

	// Recursively merge child classes
	for _, sub := range node.SubClasses {
		if err := mergeObjectClass(omt, sub, currentHandle); err != nil {
			return err
		}
	}

	return nil
}

func mergeInteractionClass(omt *core.OMTRegistry, node XMLInteractionClass, parentHandle common.InteractionClassHandle) error {
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return nil
	}

	var currentHandle common.InteractionClassHandle

	existing, exists := omt.GetInteractionClassByName(name)
	if exists {
		currentHandle = existing.Handle
	} else {
		if name == core.HLAinteractionRootName {
			currentHandle = core.HLAinteractionRootHandle
		} else {
			if parentHandle == 0 {
				parentHandle = core.HLAinteractionRootHandle
			}
			newInter, err := omt.AddInteractionClass(name, parentHandle)
			if err != nil {
				return fmt.Errorf("failed to add interaction class '%s': %w", name, err)
			}
			currentHandle = newInter.Handle
		}
	}

	// Merge parameters
	for _, param := range node.Parameters {
		paramName := strings.TrimSpace(param.Name)
		if paramName == "" {
			continue
		}
		dt := strings.TrimSpace(param.DataType)
		if dt == "" {
			dt = "HLAASCIIstring"
		}
		// Try adding parameter (AddParameter checks duplicates internally)
		if _, err := omt.AddParameter(currentHandle, paramName, dt); err != nil {
			// If already exists, ignore
			_ = err
		}
	}

	// Recursively merge child interactions
	for _, sub := range node.SubClasses {
		if err := mergeInteractionClass(omt, sub, currentHandle); err != nil {
			return err
		}
	}

	return nil
}
