package metamodel

// SUID canônico comum para os metadados do modelo de objetos do Portico (ObjectModel, OCMetadata, ACMetadata, ICMetadata, PCMetadata)
// SUID oficial: 98121116105109L (0x0000593D9A453995)
const PorticoModelSUID uint64 = 0x0000593D9A453995

var (
	ClassDescHLAVersion = &ClassDesc{
		Name:             "org.portico.impl.HLAVersion",
		SerialVersionUID: 0,
		Flags:            SC_ENUM | SC_SERIALIZABLE,
		Fields:           nil,
	}

	ClassDescSharing = &ClassDesc{
		Name:             "org.portico.lrc.model.Sharing",
		SerialVersionUID: 0,
		Flags:            SC_ENUM | SC_SERIALIZABLE,
		Fields:           nil,
	}

	ClassDescOrder = &ClassDesc{
		Name:             "org.portico.lrc.model.Order",
		SerialVersionUID: 0,
		Flags:            SC_ENUM | SC_SERIALIZABLE,
		Fields:           nil,
	}

	ClassDescTransport = &ClassDesc{
		Name:             "org.portico.lrc.model.Transport",
		SerialVersionUID: 0,
		Flags:            SC_ENUM | SC_SERIALIZABLE,
		Fields:           nil,
	}

	ClassDescObjectModel = &ClassDesc{
		Name:             "org.portico.lrc.model.ObjectModel",
		SerialVersionUID: PorticoModelSUID,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "handle", Type: TypeInt},
			{Name: "locked", Type: TypeBoolean},
			{Name: "privilegeToDelete", Type: TypeInt},
			{Name: "datatypes", Type: TypeObject, ClassName: "Ljava/util/Map;"},
			{Name: "filename", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "iclasses", Type: TypeObject, ClassName: "Ljava/util/Map;"},
			{Name: "icroot", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/ICMetadata;"},
			{Name: "oclasses", Type: TypeObject, ClassName: "Ljava/util/Map;"},
			{Name: "ocroot", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/OCMetadata;"},
			{Name: "spaces", Type: TypeObject, ClassName: "Ljava/util/Map;"},
			{Name: "version", Type: TypeObject, ClassName: "Lorg/portico/impl/HLAVersion;"},
		},
	}

	ClassDescOCMetadata = &ClassDesc{
		Name:             "org.portico.lrc.model.OCMetadata",
		SerialVersionUID: PorticoModelSUID,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "handle", Type: TypeInt},
			{Name: "attributes", Type: TypeObject, ClassName: "Ljava/util/Map;"},
			{Name: "children", Type: TypeObject, ClassName: "Ljava/util/Set;"},
			{Name: "model", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/ObjectModel;"},
			{Name: "name", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "parent", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/OCMetadata;"},
			{Name: "qualifiedName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "sharing", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Sharing;"},
			{Name: "vsafeQualifiedName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		},
	}

	ClassDescACMetadata = &ClassDesc{
		Name:             "org.portico.lrc.model.ACMetadata",
		SerialVersionUID: PorticoModelSUID,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "handle", Type: TypeInt},
			{Name: "container", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/OCMetadata;"},
			{Name: "datatype", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/datatype/IDatatype;"},
			{Name: "name", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "order", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Order;"},
			{Name: "sharing", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Sharing;"},
			{Name: "space", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Space;"},
			{Name: "transport", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Transport;"},
		},
	}

	ClassDescICMetadata = &ClassDesc{
		Name:             "org.portico.lrc.model.ICMetadata",
		SerialVersionUID: PorticoModelSUID,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "handle", Type: TypeInt},
			{Name: "children", Type: TypeObject, ClassName: "Ljava/util/Set;"},
			{Name: "model", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/ObjectModel;"},
			{Name: "name", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "order", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Order;"},
			{Name: "parameters", Type: TypeObject, ClassName: "Ljava/util/Map;"},
			{Name: "parent", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/ICMetadata;"},
			{Name: "qualifiedName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "sharing", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Sharing;"},
			{Name: "space", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Space;"},
			{Name: "transport", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/Transport;"},
			{Name: "vsafeQualifiedName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		},
	}

	ClassDescPCMetadata = &ClassDesc{
		Name:             "org.portico.lrc.model.PCMetadata",
		SerialVersionUID: PorticoModelSUID,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "handle", Type: TypeInt},
			{Name: "container", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/ICMetadata;"},
			{Name: "datatype", Type: TypeObject, ClassName: "Lorg/portico/lrc/model/datatype/IDatatype;"},
			{Name: "name", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		},
	}
)

func init() {
	RegisterClassDesc(ClassDescHLAVersion)
	RegisterClassDesc(ClassDescSharing)
	RegisterClassDesc(ClassDescOrder)
	RegisterClassDesc(ClassDescTransport)
	RegisterClassDesc(ClassDescObjectModel)
	RegisterClassDesc(ClassDescOCMetadata)
	RegisterClassDesc(ClassDescACMetadata)
	RegisterClassDesc(ClassDescICMetadata)
	RegisterClassDesc(ClassDescPCMetadata)
}

// Helper functions for Portico Enums
func EnumHLAVersion(name string) *JavaEnum {
	return NewJavaEnum(ClassDescHLAVersion, name)
}

func EnumSharing(name string) *JavaEnum {
	return NewJavaEnum(ClassDescSharing, name)
}

func EnumOrder(name string) *JavaEnum {
	return NewJavaEnum(ClassDescOrder, name)
}

func EnumTransport(name string) *JavaEnum {
	return NewJavaEnum(ClassDescTransport, name)
}
