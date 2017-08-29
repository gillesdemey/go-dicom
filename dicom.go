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
//     for _, elem := range data.Elements {
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
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/japanese"
	"io"
	"log"
)

// UID prefix provided by https://www.medicalconnections.co.uk/Free_UID
const DefaultImplementationClassUIDPrefix = "1.2.826.0.1.3680043.9.7133"

var DefaultImplementationClassUID = DefaultImplementationClassUIDPrefix + ".1.1"

const DefaultImplementationVersionName = "GODICOM_1_1"

// DicomFile represents result of parsing one DICOM file.
type DicomFile struct {
	// Elements in the file, in order of appearance.  Unlike pydicom,
	// Elements also contains meta elements (those with tag.group==2).
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

var htmlEncodingNames = map[string]string{
	"ISO_IR 126": "iso-ir-126",
	"ISO_IR 144": "iso-ir-144",
	"ISO_IR 127": "iso-ir-127",
	"ISO_IR 138": "iso-ir-138",
	"ISO_IR 13":  "iso-ir-13",
	"ISO_IR 166": "iso-ir-166",
	"ISO_IR 148": "iso-ir-148",
}

// Convert DICOM character encoding names, such as "ISO-IR 100" to golang
// decoder. It will return nil, nil for the default (7bit ASCII)
// encoding. Cf. P3.2
// D.6.2. http://dicom.nema.org/medical/dicom/2016d/output/chtml/part02/sect_D.6.2.html
func parseSpecificCharacterSet(elem *DicomElement) (CodingSystem, error) {
	// Set the []byte -> string decoder for the rest of the
	// file.  It's sad that SpecificCharacterSet isn't part
	// of metadata, but is part of regular attrs, so we need
	// to watch out for multiple occurrences of this type of
	// elements.
	encodingNames, err := GetStrings(elem)
	if err != nil {
		return CodingSystem{}, err
	}
	var decoders []*encoding.Decoder
	for _, name := range encodingNames {
		var c *encoding.Decoder
		if htmlName, ok := htmlEncodingNames[name]; ok {
			d, err := htmlindex.Get(htmlName)
			if err != nil {
				panic(err)
			}
			c = d.NewDecoder()
		}
		if c == nil {
			switch name {
			case "ISO 2022 IR 6":
				fallthrough
			case "ISO_IR 100":
				c = charmap.ISO8859_1.NewDecoder()
			case "ISO_IR 101":
				c = charmap.ISO8859_2.NewDecoder()
			case "ISO_IR 109":
				c = charmap.ISO8859_3.NewDecoder()
			case "ISO_IR 110":
				c = charmap.ISO8859_4.NewDecoder()
			case "ISO 2022 IR 13":
				c = japanese.ShiftJIS.NewDecoder()
			case "ISO 2022 IR 87":
				fallthrough
			case "ISO 2022 IR 159":
				c = japanese.ISO2022JP.NewDecoder()
				// TODO(saito) handle below.
				// case "ISO 2022 IR 100":
				// case "ISO 2022 IR 101":
				// case "ISO 2022 IR 109":
				// case "ISO 2022 IR 110":
				// case "ISO 2022 IR 144":
				// case "ISO 2022 IR 127":
				// case "ISO 2022 IR 126":
				// case "ISO 2022 IR 138":
				// case "ISO 2022 IR 148":
				// case "ISO 2022 IR 13":
				// case "ISO 2022 IR 166":
				// case "ISO 2022 IR 87":
				// case "ISO 2022 IR 159":
				// case "ISO 2022 IR 149":
			}
		}
		if c == nil {
			// TODO(saito) Support more encodings.
			log.Printf("Unknown character set '%s'. Assuming utf-8", encodingNames[0])
		}
		decoders = append(decoders, c)
	}
	if len(decoders) == 0 {
		return CodingSystem{nil, nil, nil}, nil
	} else if len(decoders) == 1 {
		return CodingSystem{decoders[0], decoders[0], decoders[0]}, nil
	} else if len(decoders) == 2 {
		return CodingSystem{decoders[0], decoders[1], decoders[1]}, nil
	} else {
		return CodingSystem{decoders[0], decoders[1], decoders[2]}, nil
	}
}

// ParseBytes(buf) is a shorthand for Parse(bytes.NewBuffer(buf), len(buf)).
func ParseBytes(data []byte) (*DicomFile, error) {
	return Parse(bytes.NewBuffer(data), int64(len(data)))
}

// Parse up to "bytes" from "io" as DICOM file. Returns a DICOM file struct
func Parse(in io.Reader, bytes int64) (*DicomFile, error) {
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
	transferSyntaxUID, err := elem.GetString()
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
			cs, err := parseSpecificCharacterSet(elem)
			if err != nil {
				buffer.SetError(err)
			} else {
				buffer.SetCodingSystem(cs)
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
	ts, err := elem.GetString()
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
