package dicom_test

import (
	"os"
	"testing"

	"github.com/yasushi-saito/go-dicom"
	"v.io/x/lib/vlog"
)

func mustReadFile(path string, options dicom.ReadOptions) *dicom.DataSet {
	data, err := dicom.ReadDataSetFromFile(path, options)
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
		_ = mustReadFile("examples/"+name, dicom.ReadOptions{})
	}
}

func TestWriteFile(t *testing.T) {
	path := "examples/IM-0001-0001.dcm"
	data := mustReadFile(path, dicom.ReadOptions{})

	dstPath := "/tmp/test.dcm"
	out, err := os.Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	err = dicom.WriteDataSet(out, data)
	if err != nil {
		t.Fatal(err)
	}
	_ = mustReadFile(dstPath, dicom.ReadOptions{})
	// TODO(saito) Fix below.
	// if !reflect.DeepEqual(data, data2) {
	// 	t.Error("Files aren't equal")
	// }
}

func TestReadDataSet(t *testing.T) {
	data := mustReadFile("examples/IM-0001-0001.dcm", dicom.ReadOptions{})
	elem, err := data.FindElementByName("PatientName")
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
	elem, err = data.FindElementByName("TransferSyntaxUID")
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
	elem, err = data.FindElementByTag(dicom.TagPixelData)
	if err != nil {
		t.Error(err)
	}
}

// Test ReadOptions.DropPixelData.
func TestDropPixelData(t *testing.T) {
	data := mustReadFile("examples/IM-0001-0001.dcm", dicom.ReadOptions{DropPixelData: true})
	_, err := data.FindElementByTag(dicom.TagPatientName)
	if err != nil {
		t.Error(err)
	}
	_, err = data.FindElementByTag(dicom.TagPixelData)
	if err == nil {
		t.Errorf("PixelData should not be present")
	}
}

func BenchmarkParseSingle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = mustReadFile("examples/IM-0001-0001.dcm", dicom.ReadOptions{})
	}
}
