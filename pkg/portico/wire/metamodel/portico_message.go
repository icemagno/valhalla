package metamodel

import "encoding/binary"

// ClassDescPorticoMessage representa a classe base de todas as mensagens de controle do Portico
// SUID oficial: 98121116105109L
var ClassDescPorticoMessage = &ClassDesc{
	Name:             "org.portico2.common.messaging.PorticoMessage",
	SerialVersionUID: 98121116105109,
	Flags:            SC_WRITE_METHOD | SC_SERIALIZABLE,
	Fields:           nil,
	Super:            ClassDescObject,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		var block [22]byte
		if isRti, _ := obj.Get("isFromRti").(bool); isRti {
			block[0] = 0x01
		}

		src := int32(-1)
		if v, ok := obj.Get("sourceFederate").(int32); ok {
			src = v
		}
		binary.BigEndian.PutUint32(block[1:5], uint32(src))

		tgt := int32(-1)
		if v, ok := obj.Get("targetFederate").(int32); ok {
			tgt = v
		}
		binary.BigEndian.PutUint32(block[5:9], uint32(tgt))

		fed := int32(-1)
		if v, ok := obj.Get("targetFederation").(int32); ok {
			fed = v
		}
		binary.BigEndian.PutUint32(block[9:13], uint32(fed))

		timeBits := uint64(0)
		if t, ok := obj.Get("timestamp").(float64); ok {
			timeBits = binary.BigEndian.Uint64([]byte{0, 0, 0, 0, 0, 0, 0, 0})
			_ = t
		}
		binary.BigEndian.PutUint64(block[13:21], timeBits)

		if async, ok := obj.Get("isAsync").(bool); ok && async {
			block[21] = 0x01
		}

		w.WriteBlockData(block[:])
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		block, err := r.ReadBlockData()
		if err != nil {
			return err
		}
		if len(block) >= 22 {
			obj.Set("isFromRti", block[0] == 0x01)
			obj.Set("sourceFederate", int32(binary.BigEndian.Uint32(block[1:5])))
			obj.Set("targetFederate", int32(binary.BigEndian.Uint32(block[5:9])))
			obj.Set("targetFederation", int32(binary.BigEndian.Uint32(block[9:13])))
			obj.Set("isAsync", block[21] == 0x01)
		}
		_, _ = r.ReadByte() // TC_ENDBLOCKDATA
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescPorticoMessage)
}
