package dicom

import (
	"fmt"
	"testing"
)

var dict Dictionary

func init() {
	dict = NewDictionary()
}

func TestGetDictEntry(t *testing.T) {
	dict := NewDictionary()
	elem, err := LookupDictionary(dict, Tag{32736, 16})
	if err != nil {
		t.Error(err)
	}
	if elem.name != "PixelData" || elem.vr != "OX" {
		t.Errorf("Wrong element name: %s", elem.name)
	}
	elem, err = LookupDictionary(dict, Tag{0, 0x1002})
	if err != nil {
		t.Error(err)
	}
	if elem.name != "EventTypeID" || elem.vr != "US" {
		t.Errorf("Wrong element name: %s", elem.name)
	}
}

// TODO: add a test for correctly splitting ranges
func TestSplitTag(t *testing.T) {
	tag, err := splitTag("(7FE0,0010)")
	if err != nil {
		t.Error(err)
	}
	if tag.Group != 0x7FE0 {
		t.Errorf("Error splitting tag. Wrong group: %#x", tag.Group)
	}
	if tag.Element != 0x0010 {
		t.Errorf("Error splitting tag. Wrong element: %#x", tag.Element)
	}

}

func BenchmarkFindMetaGroupLengthTag(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := LookupDictionary(dict, Tag{2, 0}); err != nil {
			fmt.Println(err)
		}

	}
}

func BenchmarkFindPixelDataTag(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := LookupDictionary(dict, Tag{32736, 16}); err != nil {
			fmt.Println(err)
		}

	}
}
