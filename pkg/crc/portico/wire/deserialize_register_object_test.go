package wire

import (
	"testing"
)

func TestDeserializeRegisterObject(t *testing.T) {
	data, err := NewRegisterObjectMsg(1034, "ThorObject")
	if err != nil {
		t.Fatalf("falha ao serializar RegisterObject: %v", err)
	}

	obj, err := DeserializePorticoControlMessage(data)
	if err != nil {
		t.Fatalf("falha ao deserializar RegisterObject: %v", err)
	}

	t.Logf("Deserialized object: %+v", obj)
	classHandle, ok := obj.Get("classHandle").(int32)
	t.Logf("classHandle: %v (ok=%v)", classHandle, ok)
	objectName, ok := obj.Get("objectName").(string)
	t.Logf("objectName: %v (ok=%v)", objectName, ok)

	if classHandle != 1034 {
		t.Errorf("esperava classHandle=1034, obteve %d", classHandle)
	}
	if objectName != "ThorObject" {
		t.Errorf("esperava objectName='ThorObject', obteve %q", objectName)
	}
}
