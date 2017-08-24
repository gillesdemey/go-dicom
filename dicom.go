// DICOM file parser. Example:
//
//   package main
//
//   import (
// 	"fmt"
// 	"github.com/gillesdemey/go-dicom"
// 	"os"
//   )
//
//   func main() {
//     in, err := os.Open("myfile.dcm")
//     st, err := in.Stat()
//     data, err := dicom.Parse(in, st.Size())
//     if err != nil {
//         panic(err)
//     }
//     for _, elem := range(data.Elements) {
//         fmt.Printf("%+v\n", elem)
//     }
//   }
package dicom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// DicomFile represents result of parsing one DICOM file.
type DicomFile struct {
	Elements []DicomElement
}

// ParseBytes(buf) is shorthand for Parse(bytes.NewBuffer(buf), len(buf)).
func ParseBytes(data []byte) (*DicomFile, error) {
	return Parse(bytes.NewBuffer(data), int64(len(data)))
}

// Parse up to "bytes" from "io" as DICOM file. Returns a DICOM file struct
//
// TODO(saito) Get rid of the "bytes" argument. Detect io.EOF instead.
func Parse(in io.Reader, bytes int64) (*DicomFile, error) {
	// buffer := newDicomBuffer(buff) //*di.Bytes)
	buffer := NewDecoder(in,
		bytes,
		binary.LittleEndian,
		false)
	buffer.Skip(128) // skip preamble

	// check for magic word
	if s := buffer.DecodeString(4); s != "DICM" {
		return nil, errors.New("Keyword 'DICM' not found in the header")
	}
	file := &DicomFile{}

	// (0002,0000) MetaElementGroupLength
	metaElem := ReadDataElement(buffer)
	if buffer.Error() != nil {
		return nil, buffer.Error()
	}
	if len(metaElem.Value) < 1 {
		return nil, fmt.Errorf("No value found in meta element")
	}
	metaLength, ok := metaElem.Value[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("Expect integer as metaElem.Values[0], but found '%v'", metaElem.Value[0])
	}
	if buffer.Len() <= 0 {
		return nil, fmt.Errorf("No data element found")
	}
	file.Elements = append(file.Elements, *metaElem)

	// Read meta tags
	start := buffer.Len()
	prevLen := buffer.Len()
	for start-buffer.Len() < int64(metaLength) && buffer.Error() == nil {
		elem := ReadDataElement(buffer)
		file.Elements = append(file.Elements, *elem)
		appendDataElement(file, elem)
		if buffer.Len() >= prevLen {
			panic("Failed to consume buffer")
		}
		prevLen = buffer.Len()
	}
	if buffer.Error() != nil {
		return nil, buffer.Error()
	}
	// read endianness and explicit VR
	endianess, implicit, err := file.getTransferSyntax()
	if err != nil {
		return nil, err
	}

	// modify buffer according to new TransferSyntaxUID
	buffer.bo = endianess
	buffer.implicit = implicit

	for buffer.Len() != 0 && buffer.Error() == nil {
		elem := ReadDataElement(buffer)
		if buffer.Error() != nil {
			break
		}
		appendDataElement(file, elem)
	}
	return file, buffer.Finish()
}

func doassert(x bool) {
	if !x {
		panic("doassert")
	}
}

// Append a dataElement to the DicomFile
func appendDataElement(file *DicomFile, elem *DicomElement) {
	file.Elements = append(file.Elements, *elem)

}

// Finds the SyntaxTrasnferUID and returns the endianess and implicit VR for the file
func (file *DicomFile) getTransferSyntax() (binary.ByteOrder, bool, error) {
	var err error

	elem, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		return nil, true, err
	}

	ts := elem.Value[0].(string)

	// defaults are explicit VR, little endian
	switch ts {
	case ImplicitVRLittleEndian:
		return binary.LittleEndian, true, nil
	case ExplicitVRLittleEndian:
		return binary.LittleEndian, false, nil
	case ExplicitVRBigEndian:
		return binary.BigEndian, false, nil
	default:
		return binary.LittleEndian, false, nil
	}

}

// Lookup a tag by name
func (file *DicomFile) LookupElement(name string) (*DicomElement, error) {

	for _, elem := range file.Elements {
		if elem.Name == name {
			return &elem, nil
		}
	}
	return nil, fmt.Errorf("Could not find element '%s' in dicom file", name)
}
