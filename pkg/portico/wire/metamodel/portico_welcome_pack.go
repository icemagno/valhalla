package metamodel

import "encoding/binary"

// ClassDescWelcomePack representa o pacote oficial de boas-vindas do Portico
// SUID oficial: 273669641773098452L (0x03CC42B0DA8025D4)
var ClassDescWelcomePack = &ClassDesc{
	Name:             "org.portico2.common.services.federation.msg.WelcomePack",
	SerialVersionUID: 0x03CC42B0DA8025D4,
	Flags:            SC_EXTERNALIZABLE | SC_BLOCK_DATA,
	Fields:           nil,
	Super:            ClassDescObject,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		fedHandle, _ := obj.Get("federationHandle").(int32)
		federateHandle, _ := obj.Get("federateHandle").(int32)
		fedName, _ := obj.Get("federationName").(string)
		federateName, _ := obj.Get("federateName").(string)
		federateType, _ := obj.Get("federateType").(string)

		// Escrever bloco de dados da serialização Externalizable:
		// [4B fedHandle] + [4B federateHandle] + [2B len + fedName] + [2B len + federateName] + [2B len + federateType]
		var blk []byte
		var hBuf [8]byte
		binary.BigEndian.PutUint32(hBuf[0:4], uint32(fedHandle))
		binary.BigEndian.PutUint32(hBuf[4:8], uint32(federateHandle))
		blk = append(blk, hBuf[:]...)

		appendStr := func(s string) {
			var l [2]byte
			binary.BigEndian.PutUint16(l[:], uint16(len(s)))
			blk = append(blk, l[:]...)
			blk = append(blk, []byte(s)...)
		}

		appendStr(fedName)
		appendStr(federateName)
		appendStr(federateType)

		w.WriteBlockData(blk)
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		blk, err := r.ReadBlockData()
		if err != nil {
			return err
		}
		if len(blk) >= 8 {
			obj.Set("federationHandle", int32(binary.BigEndian.Uint32(blk[0:4])))
			obj.Set("federateHandle", int32(binary.BigEndian.Uint32(blk[4:8])))

			offset := 8
			readStr := func() string {
				if offset+2 > len(blk) {
					return ""
				}
				l := int(binary.BigEndian.Uint16(blk[offset : offset+2]))
				offset += 2
				if offset+l > len(blk) {
					return ""
				}
				s := string(blk[offset : offset+l])
				offset += l
				return s
			}

			obj.Set("federationName", readStr())
			obj.Set("federateName", readStr())
			obj.Set("federateType", readStr())
		}
		_, _ = r.ReadByte() // Consumir TC_ENDBLOCKDATA
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescWelcomePack)
}

// NewWelcomePack instancia um novo objeto WelcomePack
func NewWelcomePack(fedHandle, federateHandle int32, fedName, federateName, federateType string) *JavaObject {
	obj := NewJavaObject(ClassDescWelcomePack)
	obj.Set("federationHandle", fedHandle)
	obj.Set("federateHandle", federateHandle)
	obj.Set("federationName", fedName)
	obj.Set("federateName", federateName)
	obj.Set("federateType", federateType)
	return obj
}
