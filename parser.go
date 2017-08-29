package dicom

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strings"
)

// Constants
const (
	itemSeqGroup     = 0xFFFE
	unknownGroupName = "Unknown Group"
	privateGroupName = "Private Data"
)

func (e *DicomElement) GetUInt32() (uint32, error) {
	if len(e.Value) != 1 {
		return 0, fmt.Errorf("Found %d value(s) in getuint32 (expect 1): %v", len(e.Value), e)
	}
	v, ok := e.Value[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("Uint32 value not found in %v", e)
	}
	return v, nil
}

func (e *DicomElement) GetUInt16() (uint16, error) {
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
func (e *DicomElement) GetString() (string, error) {
	if len(e.Value) != 1 {
		return "", fmt.Errorf("Found %d value(s) in getstring (expect 1): %v", len(e.Value), e.String())
	}
	v, ok := e.Value[0].(string)
	if !ok {
		return "", fmt.Errorf("string value not found in %v", e)
	}
	return v, nil
}

func GetStrings(e *DicomElement) ([]string, error) {
	var values []string
	for _, v := range e.Value {
		v, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("string value not found in %v", e.String())
		}
		values = append(values, v)
	}
	return values, nil
}

// MustGetString() is similar to GetString(), but panics on error.
//
// TODO(saito): Add other variants of MustGet<type>.
func (e *DicomElement) MustGetString() string {
	v, err := e.GetString()
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
	s = fmt.Sprintf("%s %s %s %s ", s, TagDebugString(e.Tag), e.Vr, sVl)
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
	vr, vl := readImplicit(d, tag)
	if tag == tagSequenceDelimitationItem {
		if vl != 0 {
			d.SetError(fmt.Errorf("SequenceDelimitationItem's VL != 0: %v", vl))
		}
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

// Consume the DICOM magic header and metadata elements (whose elements with tag
// group==2) from a Dicom file. Errors are reported through d.Error().
func ParseFileHeader(d *Decoder) []DicomElement {
	d.PushTransferSyntax(binary.LittleEndian, ExplicitVR)
	defer d.PopTransferSyntax()
	d.Skip(128) // skip preamble

	// check for magic word
	if s := d.DecodeString(4); s != "DICM" {
		d.SetError(errors.New("Keyword 'DICM' not found in the header"))
		return nil
	}

	// (0002,0000) MetaElementGroupLength
	metaElem := ReadDataElement(d)
	if d.Error() != nil {
		return nil
	}
	if metaElem.Tag != TagMetaElementGroupLength {
		d.SetError(fmt.Errorf("MetaElementGroupLength not found; insteadfound %s", metaElem.Tag.String()))
	}
	metaLength, err := metaElem.GetUInt32()
	if err != nil {
		d.SetError(fmt.Errorf("Failed to read uint32 in MetaElementGroupLength: %v", err))
		return nil
	}
	if d.Len() <= 0 {
		d.SetError(fmt.Errorf("No data element found"))
		return nil
	}
	metaElems := []DicomElement{*metaElem}

	// Read meta tags
	d.PushLimit(int64(metaLength))
	defer d.PopLimit()
	for d.Len() > 0 {
		elem := ReadDataElement(d)
		if d.Error() != nil {
			break
		}
		metaElems = append(metaElems, *elem)
	}
	return metaElems
}

// Read a DICOM data element. Errors are reported through d.Error(). The caller
// must check d.Error() before using the returned value.
func ReadDataElement(d *Decoder) *DicomElement {
	tag := readTag(d)
	var vr string     // Value Representation
	var vl uint32 = 0 // Value Length

	// The elements for group 0xFFFE should be Encoded as Implicit VR.
	// DICOM Standard 09. PS 3.6 - Section 7.5: "Nesting of Data Sets"
	implicit := d.implicit
	if tag.Group == itemSeqGroup {
		implicit = ImplicitVR
	}
	if implicit == ImplicitVR {
		vr, vl = readImplicit(d, tag)
	} else {
		doassert(implicit == ExplicitVR)
		vr, vl = readExplicit(d, tag)
	}
	if d.Error() != nil {
		return nil
	}
	if vr == "OX" {
		// TODO(saito) Figure out how to converct ox to a concrete
		// type. I can't find one in the spec.
		vr = "OW"
	}
	elem := &DicomElement{
		Tag: tag,
		Vr:  vr,
		Vl:  vl,
	}
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
			for {
				item := ReadDataElement(d)
				if d.Error() != nil {
					break
				}
				if item.Tag == tagSequenceDelimitationItem {
					break
				}
				if item.Tag != TagItem {
					d.SetError(fmt.Errorf("Found non-Item element in seq w/ undefined length: %v", TagDebugString(item.Tag)))
					break
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
					d.SetError(fmt.Errorf("Found non-Item element in seq w/ undefined length: %v", TagDebugString(item.Tag)))
					break
				}
				data = append(data, item)
			}
		}
	} else if vr == "NA" {
		// Parse Item.
		if vl == UndefinedLength {
			// Format: Item Any* ItemDelimitationItem
			for {
				subelem := ReadDataElement(d)
				if d.Error() != nil {
					break
				}
				if subelem.Tag == tagItemDelimitationItem {
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
			panic(fmt.Errorf("Undefined length disallowed for VR=%s, tag %s", vr, TagDebugString(tag)))
			d.SetError(fmt.Errorf("Undefined length disallowed for VR=%s, tag %s", vr, TagDebugString(tag)))
			return nil
		}
		d.PushLimit(int64(vl))
		defer d.PopLimit()
		if vr == "DA" {
			// 8-byte Date string of form 19930822 or 10-byte
			// ACR-NEMA300 string of form "1993.08.22". The latter
			// is not compliant according to P3.5 6.2, but it still
			// happens in real life.
			for d.Len() > 0 && d.Error() == nil {
				date := d.DecodeString(8)
				if strings.Contains(date, ".") {
					date += d.DecodeString(2)
				}
				data = append(data, date)
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
		} else if vr == "LT" || vr == "UT" {
			str := d.DecodeString(int(vl))
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
			str := strings.Trim(v, " \000")
			if len(str) > 0 {
				for _, s := range strings.Split(str, "\\") {
					data = append(data, s)
				}
			}
		}
	}
	elem.Value = data
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
func readImplicit(buffer *Decoder, tag Tag) (string, uint32) {
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
		buffer.SetError(fmt.Errorf("Encountered odd length (vl=%v) when reading implicit VR '%v' for tag %s", vl, vr, TagDebugString(tag)))
	}
	return vr, vl
}

// The VR is represented by the next two consecutive bytes
// The VL depends on the VR value
func readExplicit(buffer *Decoder, tag Tag) (string, uint32) {
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
		buffer.SetError(fmt.Errorf("Encountered odd length (vl=%v) when reading explicit VR %v for tag %s", vl, vr, TagDebugString(tag)))
	}
	return vr, vl
}
