package dicom_test

import (
	"encoding/binary"
	"github.com/yasushi-saito/go-dicom"
	"testing"
)

func TestEncodeDataElement(t *testing.T) {
	// Encode two scalar elements.
	e := dicom.NewEncoder(binary.LittleEndian)
	var values []interface{}
	values = append(values, string("FooHah"))
	dicom.EncodeDataElement(e, dicom.Tag{0x0018, 0x9755}, values)
	values = nil
	values = append(values, uint32(1234))
	values = append(values, uint32(2345))
	dicom.EncodeDataElement(e, dicom.Tag{0x0020, 0x9057}, values)

	data, err := e.Finish()
	if err != nil {
		t.Error(err)
	}

	// Read them back.
	d := dicom.NewBytesDecoder(data, binary.LittleEndian, true)
	elem0 := dicom.ReadDataElement(d)
	tag := dicom.Tag{Group: 0x18, Element: 0x9755}
	if elem0.Tag != tag {
		t.Error("Bad tag", elem0)
	}
	if len(elem0.Value) != 1 {
		t.Error("Bad value", elem0)
	}
	if elem0.Value[0].(string) != "FooHah" {
		t.Error("Bad value", elem0)
	}

	tag = dicom.Tag{Group: 0x20, Element: 0x9057}
	elem1 := dicom.ReadDataElement(d)
	if elem1.Tag != tag {
		t.Error("Bad tag")
	}
	if len(elem1.Value) != 2 {
		t.Error("Bad value", elem1)
	}
	if elem1.Value[0].(uint32) != 1234 {
		t.Error("Bad value", elem1)
	}
	if elem1.Value[1].(uint32) != 2345 {
		t.Error("Bad value", elem1)
	}
	if err := d.Finish(); err != nil {
		t.Error(err)
	}

}
