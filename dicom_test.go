package dicom

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"testing"
)

func readFile() (io.Reader, int64) {
	file, err := os.Open("examples/IM-0001-0001.dcm")
	if err != nil {
		panic(err)
	}
	st, err := file.Stat()
	if err != nil {
		panic(err)
	}
	return file, st.Size()
}

func TestParseFile(t *testing.T) {
	file, fileSize := readFile()

	parser := NewParser()
	data, err := parser.Parse(file, fileSize)
	if err != nil {
		t.Errorf("failed to parse dicom file: %s", err)
	}

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

func TestGetTransferSyntaxImplicitLittleEndian(t *testing.T) {
	file := &DicomFile{}
	values2 := make([]interface{}, 1)
	values2[0] = "1.2.840.10008.1.2"
	file.Elements = append(
		file.Elements,
		DicomElement{Tag{0002, 0010}, "TransferSyntaxUID", "UI", 0, values2, 0, 0})

	bo, implicit, err := file.getTransferSyntax()
	if err != nil {
		t.Errorf("Could not get TransferSyntaxUID. %s", err)
	}

	if bo != binary.LittleEndian {
		t.Errorf("Incorrect ByteOrder %v. Should be LittleEndian.", bo)
	}

	if implicit != true {
		t.Errorf("Incorrect implicitness %v. Should be true.", implicit)
	}

}

func BenchmarkParseSingle(b *testing.B) {
	parser := NewParser()
	for i := 0; i < b.N; i++ {
		file, fileSize := readFile()
		_, err := parser.Parse(file, fileSize)
		if err != nil {
			fmt.Println("failed to parse dicom file")
			panic(err)
		}
	}
}
