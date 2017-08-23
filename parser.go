package dicom

import (
	"fmt"
	"log"
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
	Tag   Tag
	Name  string // Name of "Tag", as defined in the data dictionary
	Vr    string // "AE", "UL", etc.
	Vl    uint32
	Value []interface{} // Value Multiplicity PS 3.5 6.4
	// IndentLevel uint8
	elemLen uint32 // Element length, in bytes.
	Pos     int64  // The byte position of the start of the element.
}

// P3.5 7.5
// Sequence of element.
type DicomItem []*DicomElement

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

func elementDebugString(e *DicomElement, nestLevel int) string {
	doassert(nestLevel < 10)
	s := strings.Repeat(" ", nestLevel)
	sVl := fmt.Sprintf("%d", e.Vl)
	if e.Vl == UndefinedLength {
		sVl = "UNDEF"
	}
	s = fmt.Sprintf("%08d %s (%04X, %04X) %s %s %d %s ", e.Pos, s, e.Tag.Group, e.Tag.Element, e.Vr, sVl, e.elemLen, e.Name)
	if e.Vr != "SQ" {
		sv := fmt.Sprintf("%v", e.Value)
		if len(sv) > 50 {
			sv = sv[1:50] + "(...)"
		}
		s += sv
	} else {
		s += " seq:\n"
		for _, v := range e.Value {
			item := v.(*DicomElement)
			s += elementDebugString(item, nestLevel+1)
		}
	}
	return s
}

// Stringer
func (e *DicomElement) String() string {
	return elementDebugString(e, 0)
}

// Create a new parser, with functional options for configuration
// http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
func NewParser() *Parser {
	return &Parser{}
}

var xxxx bool

// Read Item object as raw bytes. Used to parse pixel data.
func readRawItem(d *Decoder) ([]byte, bool) {
	tag := readTag(d)
	// Item is always encoded implicit. PS3.6 7.5
	_, vr, vl := readImplicit(d, tag)
	if tag == tagSequenceDelimitationItem.Tag {
		doassert(vl == 0)
		return nil, true
	}
	if tag != tagItem.Tag {
		d.SetError(fmt.Errorf("Expect item in pixeldata but found %v", tag))
		return nil, false
	}
	if vl == UndefinedLength {
		d.SetError(fmt.Errorf("Expect defined-length item in pixeldata"))
		return nil, false
	}
	doassert(vr == "NA")
	return d.DecodeBytes(int(vl)), false
}

func readBasicOffsetTable(d *Decoder) []uint32 {
	data, endOfData := readRawItem(d)
	if endOfData {
		d.SetError(fmt.Errorf("basic offset table not found"))
	}
	if len(data) == 0 {
		return []uint32{0}
	}
	subdecoder := NewBytesDecoder(data, d.bo, true)
	var offsets []uint32
	for subdecoder.Len() > 0 && subdecoder.Error() == nil {
		offsets = append(offsets, subdecoder.DecodeUInt32())
	}
	return offsets
}

// Read a DICOM data element
func ReadDataElement(d *Decoder) *DicomElement {
	initialPos := d.Pos()
	tag := readTag(d)
	var elem *DicomElement
	var vr string     // Value Representation
	var vl uint32 = 0 // Value Length

	// The elements for group 0xFFFE should be Encoded as Implicit VR.
	// DICOM Standard 09. PS 3.6 - Section 7.5: "Nesting of Data Sets"
	implicit := d.implicit
	if tag.Group == pixeldataGroup {
		implicit = true
	}
	if implicit {
		elem, vr, vl = readImplicit(d, tag)
	} else {
		elem, vr, vl = readExplicit(d, tag)
	}
	if d.Error() != nil {
		return nil
	}

	elem.Vr = vr
	elem.Vl = vl
	// log.Printf("READTAG: pos:%d, %s %s %v", d.Pos(), tag.String(), vr, vl)
	if d.Pos() == 2806 {
		xxxx = true
	}
	// data
	var data []interface{}

	if tag == TagPixelData.Tag {
		doassert(vl == UndefinedLength)
		_ = readBasicOffsetTable(d) // TODO(saito) Use the offset table.
		var bytes []byte
		for d.Len() > 0 && d.Error() == nil {
			chunk, endOfItems := readRawItem(d)
			if endOfItems {
				break
			}
			bytes = append(bytes, chunk...)
		}
		data = append(data, bytes)
		// TODO(saito) handle multi-frame image.
	} else if vr == "SQ" {
		if vl == UndefinedLength {
			for d.Len() > 0 && d.Error() == nil {
				item := ReadDataElement(d)
				if item.Tag != tagItem.Tag {
					log.Panicf("Unknown item in seq(unlimited): %v", TagDebugString(item.Tag))
				}
				data = append(data, item)
			}
		} else {
			d.PushLimit(int64(vl))
			defer d.PopLimit()
			xxxx = true
			for d.Len() > 0 && d.Error() == nil {
				item := ReadDataElement(d)
				if item.Tag != tagItem.Tag {
					log.Panicf("Unknown item in seq: %v", TagDebugString(item.Tag))
				}
				data = append(data, item)
			}
		}
	} else if vr == "NA" {
		// parse a list of Items
		if vl == UndefinedLength {
			for d.Len() > 0 && d.Error() == nil && elem.Tag != tagItemDelimitationItem.Tag {
				subelem := ReadDataElement(d)
				data = append(data, subelem)
			}
		} else {
			d.PushLimit(int64(vl))
			for d.Len() > 0 && d.Error() == nil {
				subelem := ReadDataElement(d)
				data = append(data, subelem)
			}
			d.PopLimit()
		}
	} else {
		if vl == UndefinedLength {
			d.SetError(fmt.Errorf("Undefined length found at offset %d for element with VR=%s", initialPos, vr))
			return nil
		}
		d.PushLimit(int64(vl))
		for d.Len() > 0 && d.Error() == nil {
			switch vr {
			case "AT":
				data = append(data, d.DecodeUInt16())
			case "UL":
				data = append(data, d.DecodeUInt32())
			case "SL":
				data = append(data, d.DecodeInt32())
			case "US":
				data = append(data, d.DecodeUInt16())
			case "SS":
				data = append(data, d.DecodeInt16())
			case "FL":
				data = append(data, d.DecodeFloat32())
			case "FD":
				data = append(data, d.DecodeFloat64())
			case "OW":
				data = append(data, d.DecodeBytes(int(vl)))
			case "OB":
				data = append(data, d.DecodeBytes(int(vl)))
			default:
				str := strings.TrimRight(d.DecodeString(int(vl)), " ")
				strs := strings.Split(str, "\\")
				for _, s := range strs {
					data = append(data, s)
				}

			}
		}
		d.PopLimit()
	}
	elem.Value = data
	elem.Pos = initialPos
	elem.elemLen = uint32(d.Pos() - initialPos)
	return elem
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
func readImplicit(buffer *Decoder, tag Tag) (*DicomElement, string, uint32) {
	elem := &DicomElement{
		Tag:  tag,
		Name: getTagName(tag),
	}
	vr := "UN"
	if entry, err := LookupTag(tag); err == nil {
		vr = entry.VR
	}

	vl := buffer.DecodeUInt32()
	// Rectify Undefined Length VL
	if vl == 0xffffffff {
		vl = UndefinedLength
		// elem.undefLen = true
	}
	// Error when encountering odd length
	if vl > 0 && vl%2 != 0 {
		buffer.SetError(fmt.Errorf("Encountered odd length (vl=%v) when reading implicit VR '%v'", vl, vr))
	}
	return elem, vr, vl
}

// The VR is represented by the next two consecutive bytes
// The VL depends on the VR value
func readExplicit(buffer *Decoder, tag Tag) (*DicomElement, string, uint32) {
	elem := &DicomElement{
		Tag:  tag,
		Name: getTagName(tag),
	}
	vr := buffer.DecodeString(2)
	// buffer.p += 2

	var vl uint32
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
				buffer.SetError(ErrUndefLengthNotAllowed)
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
	if vl > 0 && vl%2 != 0 {
		buffer.SetError(fmt.Errorf("Encountered odd length (vl=%v) when reading explicit VR %v", vl, vr))
	}
	return elem, vr, vl
}

// func EncodeDataElement(e *Encoder, tag Tag, value []interface{}) {
// 	e.EncodeUInt16(tag.Group)
// 	e.EncodeUInt16(tag.Element)

// 	// TODO(saito) For now, only implicit encoding is supported.
// 	vr := "UN"
// 	if entry, err := LookupTag(tag); err == nil {
// 		vr = entry.VR
// 	}
// 	if vl == UndefinedLength {
// 		// TODO: set UndefinedLength to this value.
// 		buffer.EncodeUInt32(0xffffffff)
// 	} else {
// 		buffer.EncodeUInt32(vl)
// 		if vl%2!=0{panic(vl)}
// 	}

// 	// data
// 	var data []interface{}
// 	uvl := vl
// 	valLen := uint32(vl)

// 	for vl != UndefinedLength && uvl > 0 {
// 		switch vr {
// 		case "AT":
// 			valLen = 2
// 			data = append(data, buffer.DecodeUInt16())
// 		case "UL":
// 			valLen = 4
// 			data = append(data, buffer.DecodeUInt32())
// 		case "SL":
// 			valLen = 4
// 			data = append(data, buffer.DecodeInt32())
// 		case "US":
// 			valLen = 2
// 			data = append(data, buffer.DecodeUInt16())
// 		case "SS":
// 			valLen = 2
// 			data = append(data, buffer.DecodeInt16())
// 		case "FL":
// 			valLen = 4
// 			data = append(data, buffer.DecodeFloat32())
// 		case "FD":
// 			valLen = 8
// 			data = append(data, buffer.DecodeFloat64())
// 		case "OW":
// 			valLen = vl
// 			data = append(data, buffer.DecodeBytes(int(vl)))
// 		case "OB":
// 			valLen = vl
// 			data = append(data, buffer.DecodeBytes(int(vl)))
// 		case "NA":
// 			valLen = vl
// 		//case "XS": ??

// 		case "SQ":
// 			valLen = vl
// 			data = append(data, "")
// 		default:
// 			valLen = vl
// 			str := strings.TrimRight(buffer.DecodeString(int(vl)), " ")
// 			strs := strings.Split(str, "\\")
// 			for _, s := range strs {
// 				data = append(data, s)
// 			}

// 		}
// 		uvl -= valLen
// 	}

// }

// func readSequence(d*Decoder, length uint32) []DicomItem {
// 	var items []DicomItem
// 	if length == UndefinedLength {
// 		for d.Len() != 0 && d.Error() == nil {
// 			item, endOfSequence := readOneItem(d)
// 			if endOfSequence {
// 				break
// 			}
// 			items = append(items, item)
// 		}
// 	} else {
// 		d.PushLimit(int64(length))
// 		defer d.PopLimit()
// 		for d.Len() != 0 && d.Error() == nil {
// 			item, endOfSequence := readOneItem(d)
// 			if endOfSequence {
// 				d.SetError(fmt.Errorf("Unexpected end of sequence marker found"))
// 				break
// 			}
// 			items = append(items, item)
// 		}
// 	}
// 	return items
// }

// func oldreadOneItem(d*Decoder) (DicomItem, bool) {
// 	var elems DicomItem
// 	// An item must start with an "Item" element.
// 	elem, err := ReadDataElement(d)
// 	if err != nil {
// 		d.SetError(err)
// 		return nil, false
// 	}
// 	// The SQ element must contain one Item element.
// 	if elem.Tag == tagSequenceDelimitationItem.Tag {
// 		return nil, true
// 	}
// 	if elem.Tag != tagItem.Tag {
// 		d.SetError( fmt.Errorf("Expect an Item element but found %v", elem))
// 		return nil, false
// 	}
// 	length := elem.Vl
// 	doassert(len(elem.Value)==1)
// 	itemData := elem.Value[0].([]byte)
// 	itemDecoder = NewDecoder(
// 		bytes.NewBuffer(itemData), len(itemData),
// 		d.bo, d.implicit)

// 	if length == UndefinedLength {
// 		for itemDecoder.Len() != 0 && d.Error() == nil && elem.Tag != tagItemDelimitationItem.Tag {
// 			elem, err = ReadDataElement(itemDecoder)
// 			if err != nil {
// 				d.SetError(err)
// 				break
// 			}
// 			elems = append(elems, elem)
// 		}
// 	} else {
// 		d.PushLimit(int64(length))
// 		defer d.PopLimit()
// 		for d.Len() != 0 && d.Error() == nil {
// 			elem, err = ReadDataElement(d)
// 			if err != nil {
// 				d.SetError(err)
// 				break
// 			}
// 			elems = append(elems, elem)
// 			length--
// 		}
// 	}
// 	return elems, false
// }

// func readOneItem(d*Decoder, length uint32) DicomItem {
// 	var elems DicomItem
// 	// An item must start with an "Item" element.
// 	//elem, err := ReadDataElement(d)
// 	//if err != nil {
// 	//d.SetError(err)
// 	//return nil, false
// 	//}

// 	// The SQ element must contain one Item element.
// 	//if elem.Tag == tagSequenceDelimitationItem.Tag {
// 	//return nil, true
// //}
// 	//if elem.Tag != tagItem.Tag {
// 	//d.SetError( fmt.Errorf("Expect an Item element but found %v", elem))
// 	//return nil, false
// //}
// 	return elems, false
// }
