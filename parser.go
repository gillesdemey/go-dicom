package dicom

import (
	"fmt"
	"strings"
)

// Constants
const (
	pixeldata_group    = 0xFFFE
	unknown_group_name = "Unknown Group"
	private_group_name = "Private Data"
)

// Value Multiplicity PS 3.5 6.4
type dcmVM struct {
	s   string
	Min uint8
	Max uint8
	N   bool
}

// A DICOM element
type DicomElement struct {
	Tag         Tag
	Name        string // Name of "Tag", as defined in the data dictionary
	Vr          string // "AE", "UL", etc.
	Vl          uint32
	Value       []interface{} // Value Multiplicity PS 3.5 6.4
	IndentLevel uint8
	elemLen     uint32
	// undefLen    bool
	P uint32
}

type Parser struct {
}

// Stringer
func (e *DicomElement) String() string {
	s := strings.Repeat(" ", int(e.IndentLevel)*2)
	sv := fmt.Sprintf("%v", e.Value)
	if len(sv) > 50 {
		sv = sv[1:50] + "(...)"
	}
	sVl := fmt.Sprintf("%d", e.Vl)
	if e.Vl == UndefinedLength {
		sVl = "UNDEF"
	}

	return fmt.Sprintf("%08d %s (%04X, %04X) %s %s %d %s %s", e.P, s, e.Tag.Group, e.Tag.Element, e.Vr, sVl, e.elemLen, e.Name, sv)
}

// Create a new parser, with functional options for configuration
// http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
func NewParser() *Parser {
	p := Parser{}
	return &p
}

// Read a DICOM data element
func readDataElement(buffer *dicomBuffer) (*DicomElement, error) {
	implicit := buffer.implicit
	inip := buffer.p
	tag := buffer.readTag()

	var elem *DicomElement
	var vr string     // Value Representation
	var vl uint32 = 0 // Value Length
	var err error
	// The elements for group 0xFFFE should be Encoded as Implicit VR.
	// DICOM Standard 09. PS 3.6 - Section 7.5: "Nesting of Data Sets"
	if tag.Group == pixeldata_group {
		implicit = true
	}

	if implicit {
		elem, vr, vl, err = buffer.readImplicit(tag)
		if err != nil {
			return nil, err
		}
	} else {
		elem, vr, vl, err = buffer.readExplicit(tag)
		if err != nil {
			return nil, err
		}
	}

	elem.Vr = vr
	elem.Vl = vl

	// data
	var data []interface{}
	uvl := vl
	valLen := uint32(vl)

	for vl != UndefinedLength && uvl > 0 {
		switch vr {
		case "AT":
			valLen = 2
			data = append(data, buffer.readUInt16())
		case "UL":
			valLen = 4
			data = append(data, buffer.readUInt32())
		case "SL":
			valLen = 4
			data = append(data, buffer.readInt32())
		case "US":
			valLen = 2
			data = append(data, buffer.readUInt16())
		case "SS":
			valLen = 2
			data = append(data, buffer.readInt16())
		case "FL":
			valLen = 4
			data = append(data, buffer.readFloat())
		case "FD":
			valLen = 8
			data = append(data, buffer.readFloat64())
		case "OW":
			valLen = vl
			data = append(data, buffer.readUInt16Array(vl))
		case "OB":
			valLen = vl
			data = append(data, buffer.readUInt8Array(vl))
		case "NA":
			valLen = vl
		//case "XS": ??

		case "SQ":
			valLen = vl
			data = append(data, "")
		default:
			valLen = vl
			str := strings.TrimRight(buffer.readString(vl), " ")
			strs := strings.Split(str, "\\")
			for _, s := range strs {
				data = append(data, s)
			}

		}
		uvl -= valLen
	}
	elem.P = inip
	elem.Value = data
	elem.elemLen = buffer.p - inip

	return elem, nil
}
