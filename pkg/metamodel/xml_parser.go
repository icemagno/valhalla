package metamodel

import (
	"encoding/xml"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/icemagno/valhalla/pkg/common"
)

// OMTBuilder decouples IEEE 1516-2010 FOM XML parsing from pkg/crc/core concrete types.
type OMTBuilder interface {
	EnsureObjectClass(parentHandle common.ObjectClassHandle, name string) (common.ObjectClassHandle, error)
	EnsureAttribute(classHandle common.ObjectClassHandle, name, datatype string) (common.AttributeHandle, error)
	EnsureInteractionClass(parentHandle common.InteractionClassHandle, name string) (common.InteractionClassHandle, error)
	EnsureParameter(interHandle common.InteractionClassHandle, name, datatype string) (common.ParameterHandle, error)
	EnsureDimension(name, datatype string, upperBound int64) (common.DimensionHandle, error)
}

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

func isNilOMTBuilder(omt OMTBuilder) bool {
	if omt == nil {
		return true
	}
	v := reflect.ValueOf(omt)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

// ParseFOMFile reads an IEEE 1516-2010 XML FOM file from disk and merges it into the target OMTBuilder.
func ParseFOMFile(omt OMTBuilder, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read FOM file '%s': %w", filePath, err)
	}
	return ParseFOMXML(omt, data)
}

// ParseFOMXML parses an in-memory byte slice containing an IEEE 1516-2010 XML FOM and merges
// its object classes, attributes, interactions, parameters, and dimensions into the target OMTBuilder.
func ParseFOMXML(omt OMTBuilder, data []byte) error {
	if isNilOMTBuilder(omt) {
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
			if _, err := omt.EnsureDimension(name, dim.DataType, dim.UpperBound); err != nil {
				return fmt.Errorf("failed to register dimension '%s': %w", name, err)
			}
		}
	}

	return nil
}

func mergeObjectClass(omt OMTBuilder, node XMLObjectClass, parentHandle common.ObjectClassHandle) error {
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return nil
	}

	var currentHandle common.ObjectClassHandle

	if name == common.HLAobjectRootName {
		currentHandle = common.HLAobjectRootHandle
	} else {
		if parentHandle == 0 {
			parentHandle = common.HLAobjectRootHandle
		}
		h, err := omt.EnsureObjectClass(parentHandle, name)
		if err != nil {
			return fmt.Errorf("failed to add object class '%s': %w", name, err)
		}
		currentHandle = h
	}

	// Merge attributes defined on this class
	for _, attr := range node.Attributes {
		attrName := strings.TrimSpace(attr.Name)
		if attrName == "" {
			continue
		}
		// Skip privilegeToDelete if on HLAobjectRoot as it's already pre-installed
		if name == common.HLAobjectRootName && attrName == common.HLAprivilegeToDeleteObjectName {
			continue
		}
		dt := strings.TrimSpace(attr.DataType)
		if dt == "" {
			dt = "HLAASCIIstring"
		}
		if _, err := omt.EnsureAttribute(currentHandle, attrName, dt); err != nil {
			return fmt.Errorf("failed to add attribute '%s' to class '%s': %w", attrName, name, err)
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

func mergeInteractionClass(omt OMTBuilder, node XMLInteractionClass, parentHandle common.InteractionClassHandle) error {
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return nil
	}

	var currentHandle common.InteractionClassHandle

	if name == common.HLAinteractionRootName {
		currentHandle = common.HLAinteractionRootHandle
	} else {
		if parentHandle == 0 {
			parentHandle = common.HLAinteractionRootHandle
		}
		h, err := omt.EnsureInteractionClass(parentHandle, name)
		if err != nil {
			return fmt.Errorf("failed to add interaction class '%s': %w", name, err)
		}
		currentHandle = h
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
		if _, err := omt.EnsureParameter(currentHandle, paramName, dt); err != nil {
			return fmt.Errorf("failed to add parameter '%s' to interaction '%s': %w", paramName, name, err)
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
