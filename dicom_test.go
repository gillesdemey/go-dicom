package dicom

import (
	"os"
	"testing"
	"v.io/x/lib/vlog"
)

func mustReadFile(path string) *DicomFile {
	file, err := os.Open(path)
	if err != nil {
		vlog.Fatalf("%s: failed to open", path, err)
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		vlog.Fatalf("%s: failed to stat", path, err)
	}
	data, err := Parse(file, st.Size())
	if err != nil {
		vlog.Fatalf("%s: failed to read: %v", path, err)
	}
	return data
}

func TestAllFiles(t *testing.T) {
	dir, err := os.Open("examples")
	if err != nil {
		panic(err)
	}
	names, err := dir.Readdirnames(0)
	if err != nil {
		panic(err)
	}
	for _, name := range names {
		vlog.Infof("Reading %s", name)
		_ = mustReadFile("examples/" + name)
	}
}

func TestParseFile(t *testing.T) {
	data := mustReadFile("examples/IM-0001-0001.dcm")
	elem, err := data.LookupElement("PatientName")
	if err != nil {
		t.Error(err)
	}
	pn := elem.Value[0].(string)
	if l := len(elem.Value); l != 1 {
		t.Errorf("Incorrect patient name length: %d", l)
	}
	if pn != "TOUTATIX" {
		t.Errorf("Incorrect patient name: %s", pn)
	}
	elem, err = data.LookupElement("TransferSyntaxUID")
	if err != nil {
		t.Error(err)
	}
	if len(elem.Value) != 1 {
		t.Errorf("Wrong value size %s", len(elem.Value))
	}
	ts := elem.Value[0].(string)
	if ts != "1.2.840.10008.1.2.4.91" {
		t.Errorf("Incorrect TransferSyntaxUID: %s", ts)
	}
	if l := len(data.Elements); l != 98 {
		t.Errorf("Error parsing DICOM file, wrong number of elements: %d", l)
	}
}

func BenchmarkParseSingle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = mustReadFile("examples/IM-0001-0001.dcm")
	}
}
