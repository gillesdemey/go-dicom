package dicom

import (
	"fmt"
	"strings"
)

// Constants
const (
	pixeldataGroup   = 0xFFFE
	unknownGroupName = "Unknown Group"
	privateGroupName = "Private Data"
)

// A DICOM element
type DicomElement struct {
	Tag         Tag
	Name        string // Name of "Tag", as defined in the data dictionary
	Vr          string // "AE", "UL", etc.
	Vl          uint32
	Value       []interface{} // Value Multiplicity PS 3.5 6.4
	IndentLevel uint8
	elemLen     uint32 // Element length, in bytes.
	Pos         int64  // The byte position of the start of the element.
}

func GetUInt32(e DicomElement) (uint32, error) {
	if len(e.Value) != 1 {
		return 0, fmt.Errorf("No value found in %v", e)
	}
	v, ok := e.Value[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("Uint32 value not found in %v", e)
	}
	return v, nil
}

func GetUInt16(e DicomElement) (uint16, error) {
	if len(e.Value) != 1 {
		return 0, fmt.Errorf("No value found in %v", e)
	}
	v, ok := e.Value[0].(uint16)
	if !ok {
		return 0, fmt.Errorf("Uint16 value not found in %v", e)
	}
	return v, nil
}

func GetString(e DicomElement) (string, error) {
	if len(e.Value) != 1 {
		return "", fmt.Errorf("No value found in %v", e)
	}
	v, ok := e.Value[0].(string)
	if !ok {
		return "", fmt.Errorf("string value not found in %v", e)
	}
	return v, nil
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
	return fmt.Sprintf("%08d %s (%04X, %04X) %s %s %d %s %s", e.Pos, s, e.Tag.Group, e.Tag.Element, e.Vr, sVl, e.elemLen, e.Name, sv)
}

// Create a new parser, with functional options for configuration
// http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
func NewParser() *Parser {
	return &Parser{}
}

// Read a DICOM data element
func ReadDataElement(buffer *Decoder) (*DicomElement, error) {
	implicit := buffer.implicit
	initialPos := buffer.Pos()
	tag := readTag(buffer)
	var elem *DicomElement
	var vr string     // Value Representation
	var vl uint32 = 0 // Value Length
	var err error
	// The elements for group 0xFFFE should be Encoded as Implicit VR.
	// DICOM Standard 09. PS 3.6 - Section 7.5: "Nesting of Data Sets"
	if tag.Group == pixeldataGroup {
		implicit = true
	}

	if implicit {
		elem, vr, vl, err = readImplicit(buffer, tag)
		if err != nil {
			return nil, err
		}
	} else {
		elem, vr, vl, err = readExplicit(buffer, tag)
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
			data = append(data, buffer.DecodeUInt16())
		case "UL":
			valLen = 4
			data = append(data, buffer.DecodeUInt32())
		case "SL":
			valLen = 4
			data = append(data, buffer.DecodeInt32())
		case "US":
			valLen = 2
			data = append(data, buffer.DecodeUInt16())
		case "SS":
			valLen = 2
			data = append(data, buffer.DecodeInt16())
		case "FL":
			valLen = 4
			data = append(data, buffer.DecodeFloat32())
		case "FD":
			valLen = 8
			data = append(data, buffer.DecodeFloat64())
		case "OW":
			valLen = vl
			data = append(data, buffer.DecodeBytes(int(vl)))
		case "OB":
			valLen = vl
			data = append(data, buffer.DecodeBytes(int(vl)))
		case "NA":
			valLen = vl
		//case "XS": ??

		case "SQ":
			valLen = vl
			data = append(data, "")
		default:
			valLen = vl
			str := strings.TrimRight(buffer.DecodeString(int(vl)), " ")
			strs := strings.Split(str, "\\")
			for _, s := range strs {
				data = append(data, s)
			}

		}
		uvl -= valLen
	}
	elem.Value = data
	elem.Pos = initialPos
	elem.elemLen = uint32(buffer.Pos() - initialPos)
	return elem, buffer.Error()
}

func getTagName(tag Tag) string {
	var name string
	//var name, vm, vr string
	entry, err := LookupTag(tag)
	if err != nil {
		panic(err)
		if tag.Group%2 == 0 {
			name = unknownGroupName
		} else {
			name = privateGroupName
		}
	} else {
		name = entry.Name
	}
	return name
}

const UndefinedLength uint32 = 0xfffffffe // must be even.

// Read a DICOM data element's tag value
// ie. (0002,0000)
// added  Value Multiplicity PS 3.5 6.4
func readTag(buffer *Decoder) Tag {
	group := buffer.DecodeUInt16()   // group
	element := buffer.DecodeUInt16() // element
	return Tag{group, element}
}

// Read the VR from the DICOM ditionary
// The VL is a 32-bit unsigned integer
func readImplicit(buffer *Decoder, tag Tag) (*DicomElement, string, uint32, error) {
	var vr string
	elem := &DicomElement{
		Tag:  tag,
		Name: getTagName(tag),
	}
	entry, err := LookupTag(tag)
	if err != nil {
		vr = "UN"
	} else {
		vr = entry.VR
	}

	vl := buffer.DecodeUInt32()
	// Rectify Undefined Length VL
	if vl == 0xffffffff {
		vl = UndefinedLength
		// elem.undefLen = true
	}
	// Error when encountering odd length
	if err == nil && vl > 0 && vl%2 != 0 {
		err = ErrOddLength
	}
	return elem, vr, vl, nil
}

// The VR is represented by the next two consecutive bytes
// The VL depends on the VR value
func readExplicit(buffer *Decoder, tag Tag) (*DicomElement, string, uint32, error) {
	elem := &DicomElement{
		Tag:  tag,
		Name: getTagName(tag),
	}
	vr := buffer.DecodeString(2)
	// buffer.p += 2

	var vl uint32
	var err error

	if vr == "US" {
		vl = 2
	}

	// long value representations
	switch vr {
	case "NA", "OB", "OD", "OF", "OL", "OW", "SQ", "UN", "UC", "UR", "UT":
		buffer.Skip(2) // ignore two bytes for "future use" (0000H)
		// buffer.p += 2

		vl = buffer.DecodeUInt32()
		// Rectify Undefined Length VL
		if vl == 0xffffffff {
			switch vr {
			case "UC", "UR", "UT":
				err = ErrUndefLengthNotAllowed
			default:
				vl = UndefinedLength
			}
		}
	default:
		vl = uint32(buffer.DecodeUInt16())
		// Rectify Undefined Length VL
		if vl == 0xffff {
			vl = UndefinedLength
		}
	}
	// Error when encountering odd length
	if err == nil && vl > 0 && vl%2 != 0 {
		err = ErrOddLength
	}
	return elem, vr, vl, err
}
