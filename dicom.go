// DICOM file parser. Example:
//
//   package main
//
//   import (
// 	"fmt"
// 	"github.com/yasushi-saito/go-dicom"
// 	"os"
//   )
//
//   func main() {
//     in, err := os.Open("myfile.dcm")
//     if err != nil {
//         panic(err)
//     }
//     st, err := in.Stat()
//     if err != nil {
//         panic(err)
//     }
//     data, err := dicom.Parse(in, st.Size())
//     if err != nil {
//         panic(err)
//     }
//     for _, elem := range data.Elements {
//         fmt.Printf("%+v\n", elem)
//     }
//   }
package dicom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/yasushi-saito/go-dicom/dicomio"
)

// UID prefix for go-dicom. Provided by
// https://www.medicalconnections.co.uk/Free_UID
const GoDICOMImplementationClassUIDPrefix = "1.2.826.0.1.3680043.9.7133"

var GoDICOMImplementationClassUID = GoDICOMImplementationClassUIDPrefix + ".1.1"

const GoDICOMImplementationVersionName = "GODICOM_1_1"

// DataSet represents contents of one DICOM file.
type DataSet struct {
	// Elements in the file, in order of appearance.
	//
	// Note: unlike pydicom, Elements also contains meta elements (those
	// with Tag.Group==2).
	Elements []*Element
}

func doassert(cond bool, values ...interface{}) {
	if !cond {
		var s string
		for _, value := range values {
			s += fmt.Sprintf("%v ", value)
		}
		panic(s)
	}
}

// ReadOptions defines how DataSets and Elements are parsed.
type ReadOptions struct {
	// If true, skip the PixelData element (bulk images) in ReadDataSet.
	DropPixelData bool
}

// ParseDataSetInBytes is a shorthand for Parse(bytes.NewBuffer(data), len(data)).
func ReadDataSetInBytes(data []byte, options ReadOptions) (*DataSet, error) {
	return ReadDataSet(bytes.NewBuffer(data), int64(len(data)), options)
}

// Parse a DICOM file stored in "io", up to "bytes". Returns a DICOM file struct
func ReadDataSet(in io.Reader, bytes int64, options ReadOptions) (*DataSet, error) {
	buffer := dicomio.NewDecoder(in, bytes, binary.LittleEndian, dicomio.ExplicitVR)
	metaElems := ParseFileHeader(buffer)
	if buffer.Error() != nil {
		return nil, buffer.Error()
	}
	file := &DataSet{Elements: metaElems}

	// Change the transfer syntax for the rest of the file.
	endian, implicit, err := getTransferSyntax(file)
	if err != nil {
		return nil, err
	}
	buffer.PushTransferSyntax(endian, implicit)
	defer buffer.PopTransferSyntax()

	// Read the list of elements.
	for buffer.Len() > 0 {
		elem := ReadElement(buffer, options)
		if buffer.Error() != nil {
			break
		}
		if elem == nil {
			// element is a pixel data and was dropped by options
			break
		}
		if elem.Tag == TagSpecificCharacterSet {
			// Set the []byte -> string decoder for the rest of the
			// file.  It's sad that SpecificCharacterSet isn't part
			// of metadata, but is part of regular attrs, so we need
			// to watch out for multiple occurrences of this type of
			// elements.
			encodingNames, err := elem.GetStrings()
			if err != nil {
				buffer.SetError(err)
			} else {
				// TODO(saito) SpecificCharacterSet may appear in a
				// middle of a SQ or NA.  In such case, the charset seem
				// to be scoped inside the SQ or NA. So we need to make
				// the charset a stack.
				cs, err := dicomio.ParseSpecificCharacterSet(encodingNames)
				if err != nil {
					buffer.SetError(err)
				} else {
					buffer.SetCodingSystem(cs)
				}
			}
		}
		file.Elements = append(file.Elements, elem)
	}
	return file, buffer.Finish()
}

func getTransferSyntax(ds *DataSet) (bo binary.ByteOrder, implicit dicomio.IsImplicitVR, err error) {
	elem, err := ds.LookupElementByTag(TagTransferSyntaxUID)
	if err != nil {
		return nil, dicomio.UnknownVR, err
	}
	transferSyntaxUID, err := elem.GetString()
	if err != nil {
		return nil, dicomio.UnknownVR, err
	}
	return dicomio.ParseTransferSyntaxUID(transferSyntaxUID)
}

func (f *DataSet) LookupElementByName(name string) (*Element, error) {
	return LookupElementByName(f.Elements, name)
}

func (f *DataSet) LookupElementByTag(tag Tag) (*Element, error) {
	return LookupElementByTag(f.Elements, tag)
}
