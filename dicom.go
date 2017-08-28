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
	"fmt"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"io"
	"log"
)

// UID prefix provided by https://www.medicalconnections.co.uk/Free_UID
const DefaultImplementationClassUIDPrefix = "1.2.826.0.1.3680043.9.7133"

var DefaultImplementationClassUID = DefaultImplementationClassUIDPrefix + ".1.1"

const DefaultImplementationVersionName = "GODICOM_1_1"

// DicomFile represents result of parsing one DICOM file.
type DicomFile struct {
	Elements []DicomElement
}

// A DICOM element
type DicomElement struct {
	Tag Tag
	// TODO(saito) Rename to VR, VL.

	// VR defines the encoding of Value[] in two-letter alphabets, e.g.,
	// "AE", "UL". See P3.5 6.2.
	Vr string

	// Total number of bytes in the Value[].  This is mostly meaningless for
	// the user of the library.
	Vl uint32

	// List of values in the element. Their types depends on VR:
	//
	// If Vr=="SQ", Value[i] is a *DicomElement, with Tag=TagItem.
	// If Vr=="NA" (i.e., Tag=tagItem), each Value[i] is a *DicomElement.
	//    a value's Tag can be any (including TagItem, which represents a nested Item)
	// If Vr=="OW" or "OB", then len(Value)==1, and Value[0] is []byte.
	// If Vr=="LT", then len(Value)==1, and Value[0] is []byte.
	// If Vr=="AT", then Value[] is a list of Tags.
	// If Vr=="US", Value[] is a list of uint16s
	// If Vr=="UL", Value[] is a list of uint32s
	// If Vr=="SS", Value[] is a list of int16s
	// If Vr=="SL", Value[] is a list of int32s
	// If Vr=="FL", Value[] is a list of float32s
	// If Vr=="FD", Value[] is a list of float64s
	// If Vr=="AT", Value[] is a list of Tag's.
	// Else, Value[] is a list of strings.
	Value []interface{} // Value Multiplicity PS 3.5 6.4
}

// Convert DICOM character encoding names, such as "ISO-IR 100" to golang
// decoder. Cf. P3.2
// D.6.2. http://dicom.nema.org/medical/dicom/2016d/output/chtml/part02/sect_D.6.2.html
func parseSpecificCharacterSet(name string) (*encoding.Decoder, error) {
	switch name {
	case "ISO_IR 100":
		return charmap.ISO8859_1.NewDecoder(), nil
	case "ISO_IR 101":
		return charmap.ISO8859_2.NewDecoder(), nil
	case "ISO_IR 109":
		return charmap.ISO8859_3.NewDecoder(), nil
	case "ISO_IR 110":
		return charmap.ISO8859_4.NewDecoder(), nil
	default:
		// TODO(saito) Suppor more chars.
		log.Printf("Unknown character set '%s'. Assuming utf-8", name)
		return nil, nil
	}
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
		ExplicitVR)

	metaElems := ParseFileHeader(buffer)
	if buffer.Error() != nil {
		return nil, buffer.Error()
	}
	file := &DicomFile{Elements: metaElems}

	// Change the transfer syntax for the rest of the file.
	elem, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		return nil, err
	}
	transferSyntaxUID, err := GetString(*elem)
	if err != nil {
		return nil, err
	}
	endian, implicit, err := ParseTransferSyntaxUID(transferSyntaxUID)
	if err != nil {
		return nil, err
	}
	buffer.PushTransferSyntax(endian, implicit)
	defer buffer.PopTransferSyntax()

	// Now read the list of elements.
	for buffer.Len() != 0 {
		elem := ReadDataElement(buffer)
		if buffer.Error() != nil {
			break
		}
		if elem.Tag == TagSpecificCharacterSet {
			// Set the []byte -> string decoder for the rest of the
			// file.  It's sad that SpecificCharacterSet isn't part
			// of metadata, but is part of regular attrs, so we need
			// to watch out for multiple occurrences of this type of
			// elements.
			encoderName, err := GetString(*elem)
			if err != nil {
				buffer.SetError(err)
				break
			}
			newDecoder, err := parseSpecificCharacterSet(encoderName)
			if err != nil {
				buffer.SetError(err)
			} else {
				log.Printf("Using new character set %v", encoderName)
				buffer.SetStringDecoder(newDecoder)
			}
		}
		file.Elements = append(file.Elements, *elem)
	}
	return file, buffer.Finish()
}

func doassert(x bool) {
	if !x {
		panic("doassert")
	}
}

// Finds the SyntaxTrasnferUID and returns the endianess and implicit VR for the file
func (file *DicomFile) getTransferSyntax() (binary.ByteOrder, IsImplicitVR, error) {
	var err error

	elem, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		return nil, UnknownVR, err
	}
	ts, err := GetString(*elem)
	if err != nil {
		return nil, UnknownVR, err
	}
	return ParseTransferSyntaxUID(ts)
}

// LookupElementByName finds an element with the given DicomElement.Name in
// "elems" If not found, returns an error.
func LookupElementByName(elems []DicomElement, name string) (*DicomElement, error) {
	t, err := LookupTagByName(name)
	if err != nil {
		return nil, err
	}
	for _, elem := range elems {
		if elem.Tag == t.Tag {
			return &elem, nil
		}
	}
	return nil, fmt.Errorf("Could not find element named '%s' in dicom file", name)
}

// LookupElementByTag finds an element with the given DicomElement.Tag in
// "elems" If not found, returns an error.
func LookupElementByTag(elems []DicomElement, tag Tag) (*DicomElement, error) {
	for _, elem := range elems {
		if elem.Tag == tag {
			return &elem, nil
		}
	}
	return nil, fmt.Errorf("Could not find element with tag %s in dicom file",
		tag.String())
}

// Lookup a tag by name. Depraceted. Use
// LookupElementByName(file.Elemements, name) or
// LookupElementByTag(file.Elemements, tag) instead
func (file *DicomFile) LookupElement(name string) (*DicomElement, error) {
	return LookupElementByName(file.Elements, name)
}
