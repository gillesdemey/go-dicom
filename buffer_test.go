package dicom_test

import (
	"bytes"
	"encoding/binary"
	"github.com/yasushi-saito/go-dicom"
	"io"
	"testing"
)

func TestBasic(t *testing.T) {
	e := dicom.NewEncoder(binary.BigEndian)
	e.EncodeByte(10)
	e.EncodeByte(11)
	e.EncodeUint16(0x123)
	e.EncodeUint32(0x234)
	e.EncodeZeros(12)
	e.EncodeString("abcde")

	encoded, err := e.Finish()
	if err != nil {
		t.Error(encoded)
	}
	d := dicom.NewDecoder(
		bytes.NewBuffer(encoded), int64(len(encoded)),
		binary.BigEndian, true)
	if v := d.DecodeByte(); v != 10 {
		t.Errorf("DecodeByte %v", v)
	}
	if v := d.DecodeByte(); v != 11 {
		t.Errorf("DecodeByte %v", v)
	}
	if v := d.DecodeUInt16(); v != 0x123 {
		t.Errorf("DecodeUint16 %v", v)
	}
	if v := d.DecodeUInt32(); v != 0x234 {
		t.Errorf("DecodeUint32 %v", v)
	}
	d.Skip(12)
	if v := d.DecodeString(5); v != "abcde" {
		t.Errorf("DecodeString %v", v)
	}
	if d.Len() != 0 {
		t.Errorf("Len %d", d.Len())
	}
	if d.Error() != nil {
		t.Errorf("!Error %v", d.Error())
	}
	// Read past the buffer. It should flag an error
	if _ = d.DecodeByte(); d.Error() == nil {
		t.Errorf("Error %v %v", d.Error())
	}
}

func TestPartialData(t *testing.T) {
	e := dicom.NewEncoder(binary.BigEndian)
	e.EncodeByte(10)
	encoded, err := e.Finish()
	if err != nil {
		t.Error(encoded)
	}
	// Read uint16, when there's only one byte in buffer.
	d := dicom.NewDecoder(bytes.NewBuffer(encoded), int64(len(encoded)),
		binary.BigEndian, true)
	if _ = d.DecodeUInt16(); d.Error() == nil {
		t.Errorf("DecodeUint16")
	}
}

func TestLimit(t *testing.T) {
	e := dicom.NewEncoder(binary.BigEndian)
	e.EncodeByte(10)
	e.EncodeByte(11)
	e.EncodeByte(12)
	encoded, err := e.Finish()
	if err != nil {
		t.Error(encoded)
	}
	// Allow reading only the first two bytes
	d := dicom.NewDecoder(bytes.NewBuffer(encoded), int64(len(encoded)),
		binary.BigEndian, true)
	if d.Len() != 3 {
		t.Errorf("Len %d", d.Len())
	}
	d.PushLimit(2)
	if d.Len() != 2 {
		t.Errorf("Len %d", d.Len())
	}
	v0, v1 := d.DecodeByte(), d.DecodeByte()
	if d.Len() != 0 {
		t.Errorf("Len %d", d.Len())
	}
	_ = d.DecodeByte()
	if v0 != 10 || v1 != 11 || d.Error() != io.EOF {
		t.Error("Limit: %v %v %v", v0, v1, d.Error())
	}

}
