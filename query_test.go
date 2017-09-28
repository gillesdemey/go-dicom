package dicom_test

import (
	"github.com/yasushi-saito/go-dicom"
	"testing"
)

func TestParse0(t *testing.T) {
	ds, err := dicom.ReadDataSetFromFile("examples/I_000013.dcm", dicom.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	studyUID :="1.2.840.113857.1907.192833.1115.220048"
	match, elem, err := dicom.Query(ds, dicom.MustNewElement(dicom.TagStudyInstanceUID, studyUID))
	if !match || err != nil {
		t.Error(err)
	}
	if elem.MustGetString() != studyUID {
		t.Error(elem)
	}
}
