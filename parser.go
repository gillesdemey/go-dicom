package dicom

import (
	"errors"
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
	Tag  Tag
	Name string // Name of "Tag", as defined in the data dictionary
	Vr   string // "AE", "UL", etc.
	Vl   uint32

	// Value encoding:
	//
	// If Vr is "SQ", Value[i] is a *DicomElement of type tagItem.
	// If Vr is "OW" or "OB", then Value[i] is raw []byte.
	// If Vr is "NA" (i.e., Tag=tagItem), each Value[i] is a *DicomElement.
	//
	// Else, Value[i] is a scalar value as defined by Vr. E.g., if Vr==UL, then each value is uint32.
	Value []interface{} // Value Multiplicity PS 3.5 6.4

	// IndentLevel uint8
	elemLen uint32 // Element length, in bytes.
	Pos     int64  // The byte position of the start of the element.
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

// GetString() is a convenience function for getting a string value from an
// element.  It returns an error the element contains more than one value, or
// the value is not a string.
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

// Read an Item object as raw bytes, w/o parsing them into DataElement. Used to
// parse pixel data.
func readRawItem(d *Decoder) ([]byte, bool) {
	tag := readTag(d)
	// Item is always encoded implicit. PS3.6 7.5
	_, vr, vl := readImplicit(d, tag)
	if tag == tagSequenceDelimitationItem {
		doassert(vl == 0)
		return nil, true
	}
	if tag != tagItem {
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

// Read the basic offset table. This is the first Item object embedded inside
// PixelData element. P3.5 8.2. P3.5, A4 has a better example.
func readBasicOffsetTable(d *Decoder) []uint32 {
	data, endOfData := readRawItem(d)
	if endOfData {
		d.SetError(fmt.Errorf("basic offset table not found"))
	}
	if len(data) == 0 {
		return []uint32{0}
	}
	// The payload of the item is sequence of uint32s, each representing the
	// byte size of an image that follows.
	subdecoder := NewBytesDecoder(data, d.bo, true)
	var offsets []uint32
	for subdecoder.Len() > 0 && subdecoder.Error() == nil {
		offsets = append(offsets, subdecoder.DecodeUInt32())
	}
	return offsets
}

// Read a DICOM data element. Errors are reported through d.Error(). The caller
// must check d.Error() before using the returned value.
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
	var data []interface{}

	if tag == TagPixelData {
		// P3.5, A.4 describes the format. Currently we only support an encapsulated image format.
		//
		// PixelData is usually the last element in a DICOM file. When
		// the file stores N images, the elements that follow PixelData
		// are laid out in the following way:
		//
		// Item(BasicOffsetTable) Item(ImageData0) ... Item(ImageDataM) SequenceDelimiterItem
		//
		// Item(BasicOffsetTable) is an Item element whose payload
		// encodes N uint32 values. Kth uint32 is the bytesize of the
		// Kth image. Item(ImageData*) are chunked sequences of bytes. I
		// presume that single ImageData item doesn't cross a image
		// boundary, but the spec isn't clear.
		//
		// The total byte size of Item(ImageData*) equal the total of
		// the bytesizes found in BasicOffsetTable.
		if vl == UndefinedLength {
			offsets := readBasicOffsetTable(d) // TODO(saito) Use the offset table.
			if len(offsets) > 1 {
				log.Printf("Warning: multiple images not supported yet. Combining them into a byte sequence: %v", offsets)
			}
			var bytes []byte
			for d.Len() > 0 && d.Error() == nil {
				chunk, endOfItems := readRawItem(d)
				if endOfItems {
					break
				}
				bytes = append(bytes, chunk...)
			}
			data = append(data, bytes)
		} else {
			log.Printf("Warning: defined-length pixel data not supported: tag %v, VR=%v, VL=%v", tag.String(), vr, vl)
			data = append(data, d.DecodeBytes(int(vl)))
		}
		// TODO(saito) handle multi-frame image.
	} else if vr == "SQ" {
		if vl == UndefinedLength {
			// Format:
			//  Sequence := ItemSet* SequenceDelimitationItem
			//  ItemSet := Item Any* ItemDelimitationItem (when Item.VL is undefined) or
			//             Item Any*N                     (when Item.VL has a defined value)
			for d.Len() > 0 && d.Error() == nil {
				item := ReadDataElement(d)
				if item.Tag == tagSequenceDelimitationItem {
					break
				}
				if item.Tag != tagItem {
					log.Panicf("Unknown item in seq(unlimited): %v", TagDebugString(item.Tag))
				}
				data = append(data, item)
			}
		} else {
			// Format:
			//  Sequence := ItemSet*VL
			// See the above comment for the definition of ItemSet.
			d.PushLimit(int64(vl))
			defer d.PopLimit()
			for d.Len() > 0 && d.Error() == nil {
				item := ReadDataElement(d)
				if item.Tag != tagItem {
					log.Panicf("Unknown item in seq: %v", TagDebugString(item.Tag))
				}
				data = append(data, item)
			}
		}
	} else if vr == "NA" {
		// Parse Item.
		if vl == UndefinedLength {
			// Format: Item Any* ItemDelimitationItem
			for d.Len() > 0 && d.Error() == nil && elem.Tag != tagItemDelimitationItem {
				subelem := ReadDataElement(d)
				data = append(data, subelem)
			}
		} else {
			// Sequence of arbitary elements, for the  total of "vl" bytes.
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
			case "UL":
				data = append(data, d.DecodeUInt32())
			case "SL":
				data = append(data, d.DecodeInt32())
			case "AT":
				fallthrough
			case "US":
				data = append(data, d.DecodeUInt16())
			case "SS":
				data = append(data, d.DecodeInt16())
			case "FL":
				data = append(data, d.DecodeFloat32())
			case "FD":
				data = append(data, d.DecodeFloat64())
			case "OW":
				fallthrough // TODO(saito) Check that size is even. Byte swap??
			case "OB":
				data = append(data, d.DecodeBytes(int(vl)))
			default:
				// String may have '\0' suffix if its length is odd.
				str := strings.TrimRight(d.DecodeString(int(vl)), " \000")
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
				buffer.SetError(errors.New("UC, UR and UT may not have an Undefined Length, i.e.,a Value Length of FFFFFFFFH."))
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

// EncodeDataElement encodes one data element. "tag" must be for a scalar
// value. That is, SQ elements are not supported yet. Errors are reported
// through e.Error() and/or E.Finish().
//
// REQUIRES: Each value in values[] must match the VR of the tag. E.g., if tag
// is for UL, then each value must be uint32.
func EncodeDataElement(e *Encoder, tag Tag, values []interface{}) {
	// TODO(saito) For now, only implicit encoding is supported.
	//  First encode the payload
	vr := "UN"
	if entry, err := LookupTag(tag); err == nil {
		vr = entry.VR
	}

	sube := NewEncoder(e.bo)
	for _, value := range values {
		switch vr {
		case "AT":
			fallthrough
		case "US":
			sube.EncodeUInt16(value.(uint16))
		case "UL":
			sube.EncodeUInt32(value.(uint32))
		case "SL":
			sube.EncodeInt32(value.(int32))
		case "SS":
			sube.EncodeInt16(value.(int16))
		case "FL":
			sube.EncodeFloat32(value.(float32))
		case "FD":
			sube.EncodeFloat64(value.(float64))
		case "OW":
			fallthrough // TODO(saito) Check that size is even. Byte swap??
		case "OB":
			sube.EncodeBytes(value.([]byte))
		case "NA":
			fallthrough
		case "SQ":
			sube.SetError(fmt.Errorf("NA and SQ encoding not supported yet"))
		default:
			{
				s := value.(string)
				sube.EncodeString(s)
				if len(s)%2 == 1 {
					sube.EncodeByte(0)
				}
			}
		}
	}
	bytes, err := sube.Finish()
	if err != nil {
		e.SetError(err)
		return
	}
	e.EncodeUInt16(tag.Group)
	e.EncodeUInt16(tag.Element)
	e.EncodeUInt32(uint32(len(bytes)))
	e.EncodeBytes(bytes)
}
