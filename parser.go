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
	Tag Tag
	// TODO(saito) Rename to VR, VL.
	Vr string // Encoding of Value. "AE", "UL", etc.
	Vl uint32

	// Value encoding:
	//
	// If Vr is "SQ", Value[i] is a *DicomElement of type tagItem.
	// If Vr is "OW" or "OB", then Value[i] is raw []byte.
	// If Vr is "NA" (i.e., Tag=tagItem), each Value[i] is a *DicomElement.
	//
	// Else, Value[i] is a scalar value as defined by Vr. E.g., if Vr==UL, then each value is uint32.
	Value []interface{} // Value Multiplicity PS 3.5 6.4

	// IndentLevel uint8
	// elemLen uint32 // Element length, in bytes.
	Pos int64 // The byte position of the start of the element.
}

func GetUInt32(e DicomElement) (uint32, error) {
	if len(e.Value) != 1 {
		return 0, fmt.Errorf("Found %d value(s) in getuint32 (expect 1): %v", len(e.Value), e)
	}
	v, ok := e.Value[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("Uint32 value not found in %v", e)
	}
	return v, nil
}

func GetUInt16(e DicomElement) (uint16, error) {
	if len(e.Value) != 1 {
		return 0, fmt.Errorf("Found %d value(s) in getuint16 (expect 1): %v", len(e.Value), e)
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
		return "", fmt.Errorf("Found %d value(s) in getstring (expect 1): %v", len(e.Value), e)
	}
	v, ok := e.Value[0].(string)
	if !ok {
		return "", fmt.Errorf("string value not found in %v", e)
	}
	return v, nil
}

// MustGetString() is similar to GetString(), but panics on error.
//
// TODO(saito): Add other variants of MustGet<type>.
func MustGetString(e DicomElement) string {
	v, err := GetString(e)
	if err != nil {
		panic(err)
	}
	return v
}

func elementDebugString(e *DicomElement, nestLevel int) string {
	doassert(nestLevel < 10)
	s := strings.Repeat(" ", nestLevel)
	sVl := fmt.Sprintf("%d", e.Vl)
	if e.Vl == UndefinedLength {
		sVl = "UNDEF"
	}
	s = fmt.Sprintf("%08d %s %s %s %s ", e.Pos, s, TagDebugString(e.Tag), e.Vr, sVl)
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
	if tag != TagItem {
		d.SetError(fmt.Errorf("Expect item in pixeldata but found %v", tag))
		return nil, false
	}
	if vl == UndefinedLength {
		d.SetError(fmt.Errorf("Expect defined-length item in pixeldata"))
		return nil, false
	}
	if vr != "NA" {
		d.SetError(fmt.Errorf("Expect NA item, but found %s", vr))
		return nil, true
	}
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
	subdecoder := NewBytesDecoder(data, d.bo, ImplicitVR)
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
		implicit = ImplicitVR
	}
	if implicit == ImplicitVR {
		elem, vr, vl = readImplicit(d, tag)
	} else {
		doassert(implicit == ExplicitVR)
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
			for d.Len() > 0 {
				chunk, endOfItems := readRawItem(d)
				if d.Error() != nil {
					break
				}
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
			for d.Len() > 0 {
				item := ReadDataElement(d)
				if d.Error() != nil {
					break
				}
				if item.Tag == tagSequenceDelimitationItem {
					break
				}
				if item.Tag != TagItem {
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
			for d.Len() > 0 {
				item := ReadDataElement(d)
				if d.Error() != nil {
					break
				}
				if item.Tag != TagItem {
					d.SetError(fmt.Errorf("Unknown item in seq: %v", TagDebugString(item.Tag)))
					log.Panic(d.Error()) // TODO: remove
				}
				data = append(data, item)
			}
		}
	} else if vr == "NA" {
		// Parse Item.
		if vl == UndefinedLength {
			// Format: Item Any* ItemDelimitationItem
			for d.Len() > 0 && elem.Tag != tagItemDelimitationItem {
				subelem := ReadDataElement(d)
				if d.Error() != nil {
					break
				}
				data = append(data, subelem)
			}
		} else {
			// Sequence of arbitary elements, for the  total of "vl" bytes.
			d.PushLimit(int64(vl))
			for d.Len() > 0 {
				subelem := ReadDataElement(d)
				if d.Error() != nil {
					break
				}
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

		if vr == "DA" {
			// 8-byte Date string
			for d.Len() > 0 && d.Error() == nil {
				data = append(data, d.DecodeString(8))
			}
		} else if vr == "AT" {
			// (2byte group, 2byte elem)
			for d.Len() > 0 && d.Error() == nil {
				tag := Tag{d.DecodeUInt16(), d.DecodeUInt16()}
				data = append(data, tag)
			}
		} else if vr == "OW" || vr == "OB" {
			// TODO(saito) Check that size is even. Byte swap??
			// TODO(saito) If OB's length is odd, is VL odd too? Need to check!
			data = append(data, d.DecodeBytes(int(vl)))
		} else if vr == "DS" || vr == "IS" {
			// Decimal string
			str := strings.Trim(d.DecodeString(int(vl)), " ")
			for _, s := range strings.Split(str, "\\") {
				data = append(data, s)
			}
		} else if vr == "LT" {
			str := d.DecodeString(int(vl))
			data = append(data, str)
		} else if vr == "LO" {
			str := strings.Trim(d.DecodeString(int(vl)), " \000")
			data = append(data, str)
		} else if vr == "UL" {
			for d.Len() > 0 && d.Error() == nil {
				data = append(data, d.DecodeUInt32())
			}
		} else if vr == "SL" {
			for d.Len() > 0 && d.Error() == nil {
				data = append(data, d.DecodeInt32())
			}
		} else if vr == "US" {
			for d.Len() > 0 && d.Error() == nil {
				data = append(data, d.DecodeUInt16())
			}
		} else if vr == "SS" {
			for d.Len() > 0 && d.Error() == nil {
				data = append(data, d.DecodeInt16())
			}
		} else if vr == "FL" {
			for d.Len() > 0 && d.Error() == nil {
				data = append(data, d.DecodeFloat32())
			}
		} else if vr == "FD" {
			for d.Len() > 0 && d.Error() == nil {
				data = append(data, d.DecodeFloat64())
			}
		} else {
			// List of strings, each delimited by '\\'.
			v := d.DecodeString(int(vl))
			// String may have '\0' suffix if its length is odd.
			str := strings.TrimRight(v, " \000")
			if len(str) > 0 {
				for _, s := range strings.Split(str, "\\") {
					data = append(data, s)
					if elem.Tag.Group == 8 && elem.Tag.Element == 0x50 {
						log.Printf("SHTAGXX2: append [%s]", s)
					}
				}
				if elem.Tag.Group == 8 && elem.Tag.Element == 0x50 {
					log.Printf("SHTAGXX: %v [%v]", len(data), data)
				}
			}
		}
		d.PopLimit()
	}
	elem.Value = data
	elem.Pos = initialPos
	return elem
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
		Tag: tag,
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
		Tag: tag,
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
func EncodeDataElement(e *Encoder, elem *DicomElement) {
	vr := elem.Vr
	if elem.Vl == UndefinedLength {
		log.Panicf("Encoding undefined-length element not yet supported: %v", elem)
	}
	entry, err := LookupTag(elem.Tag)
	if vr == "" {
		if err == nil {
			vr = entry.VR
		} else {
			vr = "UN"
		}
	} else {
		if err == nil && entry.VR != vr {
			e.SetError(fmt.Errorf("VR value mismatch. DicomElement.Vr=%v, but tag is for %v",
				vr, entry.VR))
			return
		}
	}
	doassert(vr != "")
	sube := NewEncoder(e.TransferSyntax())
	for _, value := range elem.Value {
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
			bytes := value.([]byte)
			sube.EncodeBytes(bytes)
			if len(bytes)%2 == 1 {
				sube.EncodeByte(0)
			}
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
	doassert(len(bytes)%2 == 0)
	e.EncodeUInt16(elem.Tag.Group)
	e.EncodeUInt16(elem.Tag.Element)
	if _, implicit := e.TransferSyntax(); implicit == ExplicitVR {
		doassert(len(vr) == 2)
		e.EncodeString(vr)
		switch vr {
		case "NA", "OB", "OD", "OF", "OL", "OW", "SQ", "UN", "UC", "UR", "UT":
			e.EncodeZeros(2) // two bytes for "future use" (0000H)
			e.EncodeUInt32(uint32(len(bytes)))
		default:
			e.EncodeUInt16(uint16(len(bytes)))
		}

	} else {
		if _, implicit := e.TransferSyntax(); implicit != ImplicitVR {
			log.Panicf("Unknown VR: %v", e)
		}
		e.EncodeUInt32(uint32(len(bytes)))
	}
	e.EncodeBytes(bytes)
}
