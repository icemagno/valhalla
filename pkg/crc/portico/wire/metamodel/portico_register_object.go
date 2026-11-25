package metamodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// ClassDescRegisterObject represents the official Portico object registration request
// Canonical Portico SUID: 98121116105109L (Externalizable)
var ClassDescRegisterObject = &ClassDesc{
	Name:             "org.portico2.common.services.object.msg.RegisterObject",
	SerialVersionUID: 98121116105109,
	Flags:            SC_EXTERNALIZABLE | SC_BLOCK_DATA,
	Fields:           nil,
	Super:            ClassDescPorticoMessage,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		var buf bytes.Buffer
		// super.writeExternal:
		isFromRti := false
		if v, ok := obj.Get("isFromRti").(bool); ok {
			isFromRti = v
		}
		if isFromRti {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}

		var sf int32 = -1
		if v, ok := obj.Get("sourceFederate").(int32); ok {
			sf = v
		}
		_ = binary.Write(&buf, binary.BigEndian, sf)

		var tf int32 = -1
		if v, ok := obj.Get("targetFederate").(int32); ok {
			tf = v
		}
		_ = binary.Write(&buf, binary.BigEndian, tf)

		var tfed int32 = -1
		if v, ok := obj.Get("targetFederation").(int32); ok {
			tfed = v
		}
		_ = binary.Write(&buf, binary.BigEndian, tfed)

		var ts float64 = -1.0
		if v, ok := obj.Get("timestamp").(float64); ok {
			ts = v
		}
		_ = binary.Write(&buf, binary.BigEndian, ts)

		immediate := false
		if v, ok := obj.Get("immediate").(bool); ok {
			immediate = v
		}
		if immediate {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}

		// RegisterObject.writeExternal:
		var ch int32
		if v, ok := obj.Get("classHandle").(int32); ok {
			ch = v
		}
		_ = binary.Write(&buf, binary.BigEndian, ch)

		on := "null"
		if v, ok := obj.Get("objectName").(string); ok && v != "" {
			on = v
		}
		onBytes := []byte(on)
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(onBytes)))
		buf.Write(onBytes)

		// signal = 0 (sem coleções DDM)
		buf.WriteByte(0)

		w.WriteBlockData(buf.Bytes())
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		blk, err := r.ReadBlockData()
		if err != nil {
			return err
		}
		if len(blk) < 26 {
			return fmt.Errorf("bloco curto demais para RegisterObject: %d bytes", len(blk))
		}

		// Super fields
		obj.Set("isFromRti", blk[0] != 0)
		obj.Set("sourceFederate", int32(binary.BigEndian.Uint32(blk[1:5])))
		obj.Set("targetFederate", int32(binary.BigEndian.Uint32(blk[5:9])))
		obj.Set("targetFederation", int32(binary.BigEndian.Uint32(blk[9:13])))
		tsBits := binary.BigEndian.Uint64(blk[13:21])
		obj.Set("timestamp", math.Float64frombits(tsBits))
		obj.Set("immediate", blk[21] != 0)

		// RegisterObject fields
		classHandle := int32(binary.BigEndian.Uint32(blk[22:26]))
		obj.Set("classHandle", classHandle)

		idx := 26
		if idx+2 <= len(blk) {
			strLen := int(binary.BigEndian.Uint16(blk[idx : idx+2]))
			idx += 2
			if idx+strLen <= len(blk) {
				name := string(blk[idx : idx+strLen])
				if name == "null" {
					name = ""
				}
				obj.Set("objectName", name)
				idx += strLen
			}
		}
		if idx < len(blk) {
			signal := blk[idx]
			obj.Set("signal", signal)
			idx++
		}

		// Consumir TC_ENDBLOCKDATA se presente
		b, err := r.ReadByte()
		if err == nil && b != TC_ENDBLOCKDATA {
			// se não for ENDBLOCKDATA, deixa estar
		}

		return nil
	},
}

// ClassDescDiscoverObject represents the official Portico object discovery notification
// Canonical Portico SUID: 98121116105109L
var ClassDescDiscoverObject = &ClassDesc{
	Name:             "org.portico2.common.services.object.msg.DiscoverObject",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "objectHandle", Type: TypeInt},
		{Name: "classHandle", Type: TypeInt},
		{Name: "objectName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
	},
	Super: ClassDescPorticoMessage,
}

// ClassDescDeleteObject represents the official Portico object deletion request
// Canonical Portico SUID: 98121116105109L
var ClassDescDeleteObject = &ClassDesc{
	Name:             "org.portico2.common.services.object.msg.DeleteObject",
	SerialVersionUID: 98121116105109,
	Flags:            SC_EXTERNALIZABLE | SC_BLOCK_DATA,
	Fields:           nil,
	Super:            ClassDescPorticoMessage,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		var buf bytes.Buffer
		// super:
		buf.WriteByte(0)
		_ = binary.Write(&buf, binary.BigEndian, int32(-1))
		_ = binary.Write(&buf, binary.BigEndian, int32(-1))
		_ = binary.Write(&buf, binary.BigEndian, int32(-1))
		_ = binary.Write(&buf, binary.BigEndian, float64(-1.0))
		buf.WriteByte(0)

		var oh int32
		if v, ok := obj.Get("objectHandle").(int32); ok {
			oh = v
		}
		_ = binary.Write(&buf, binary.BigEndian, oh)

		tagStr := ""
		if t, ok := obj.Get("tag").([]byte); ok {
			tagStr = string(t)
		} else if s, ok := obj.Get("tag").(string); ok {
			tagStr = s
		}
		tagBytes := []byte(tagStr)
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(tagBytes)))
		buf.Write(tagBytes)

		w.WriteBlockData(buf.Bytes())
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		blk, err := r.ReadBlockData()
		if err != nil {
			return err
		}
		if len(blk) < 26 {
			return fmt.Errorf("bloco curto demais para DeleteObject: %d bytes", len(blk))
		}
		obj.Set("isFromRti", blk[0] != 0)
		obj.Set("sourceFederate", int32(binary.BigEndian.Uint32(blk[1:5])))
		obj.Set("targetFederate", int32(binary.BigEndian.Uint32(blk[5:9])))
		obj.Set("targetFederation", int32(binary.BigEndian.Uint32(blk[9:13])))
		tsBits := binary.BigEndian.Uint64(blk[13:21])
		obj.Set("timestamp", math.Float64frombits(tsBits))
		obj.Set("immediate", blk[21] != 0)

		objHandle := int32(binary.BigEndian.Uint32(blk[22:26]))
		obj.Set("objectHandle", objHandle)

		idx := 26
		if idx+2 <= len(blk) {
			strLen := int(binary.BigEndian.Uint16(blk[idx : idx+2]))
			idx += 2
			if idx+strLen <= len(blk) {
				tag := string(blk[idx : idx+strLen])
				obj.Set("tag", []byte(tag))
			}
		}

		b, err := r.ReadByte()
		if err == nil && b != TC_ENDBLOCKDATA {
		}
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescRegisterObject)
	RegisterClassDesc(ClassDescDiscoverObject)
	RegisterClassDesc(ClassDescDeleteObject)
}

// NewRegisterObject builds a RegisterObject request
func NewRegisterObject(classHandle int32, objectName string) *JavaObject {
	obj := NewJavaObject(ClassDescRegisterObject)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("classHandle", classHandle)
	obj.Set("objectName", objectName)
	return obj
}

// NewDiscoverObject builds a DiscoverObject notification ready for delivery to federates
func NewDiscoverObject(objectHandle, classHandle int32, objectName string) *JavaObject {
	obj := NewJavaObject(ClassDescDiscoverObject)
	obj.Set("immediate", true)
	obj.Set("isFromRti", true)
	obj.Set("sourceFederate", int32(0))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("objectHandle", objectHandle)
	obj.Set("classHandle", classHandle)
	obj.Set("objectName", objectName)
	return obj
}
