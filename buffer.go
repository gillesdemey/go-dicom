package dicom

import (
	"bytes"
	"encoding/binary"
)

type dicomBuffer struct {
	*bytes.Buffer
	bo       binary.ByteOrder
	implicit bool
	p        uint32 // element start position
}

// The default DicomBuffer reads a buffer with Little Endian byteorder
// and explicit VR
func newDicomBuffer(b []byte) *dicomBuffer {
	return &dicomBuffer{
		bytes.NewBuffer(b),
		binary.LittleEndian,
		false,
		0,
	}
}

// Read the VR from the DICOM ditionary
// The VL is a 32-bit unsigned integer
func (buffer *dicomBuffer) readImplicit(elem *DicomElement, p *Parser) (string, uint32, error) {
	var vr string

	entry, err := LookupDictionary(p.dictionary, Tag{elem.Group, elem.Element})
	if err != nil {
		vr = "UN"
	} else {
		vr = entry.vr
	}

	vl := buffer.readUInt32()
	// Rectify Undefined Length VL
	if vl == 0xffffffff {
		vl = 0
		elem.undefLen = true
	}
	// Error when encountering odd length
	if err == nil && vl > 0 && vl%2 != 0 {
		err = ErrOddLength
	}
	return vr, vl, nil
}

// The VR is represented by the next two consecutive bytes
// The VL depends on the VR value
func (buffer *dicomBuffer) readExplicit(elem *DicomElement) (string, uint32, error) {
	vr := string(buffer.Next(2))
	buffer.p += 2

	var vl uint32
	var err error

	if vr == "US" {
		vl = 2
	}

	// long value representations
	switch vr {
	case "NA", "OB", "OD", "OF", "OL", "OW", "SQ", "UN", "UC", "UR", "UT":
		buffer.Next(2) // ignore two bytes for "future use" (0000H)
		buffer.p += 2

		vl = buffer.readUInt32()
		// Rectify Undefined Length VL
		if vl == 0xffffffff {
			switch vr {
			case "UC", "UR", "UT":
				err = ErrUndefLengthNotAllowed
			default:
				elem.undefLen = true
				vl = 0
			}
		}
	default:
		vl = uint32(buffer.readUInt16())
		// Rectify Undefined Length VL
		if vl == 0xffff {
			vl = 0
			elem.undefLen = true
		}
	}
	// Error when encountering odd length
	if err == nil && vl > 0 && vl%2 != 0 {
		err = ErrOddLength
	}
	return vr, vl, err
}

// Read a DICOM data element's tag value
// ie. (0002,0000)
// added  Value Multiplicity PS 3.5 6.4
func (buffer *dicomBuffer) readTag(p *Parser) *DicomElement {
	group := buffer.readUInt16()   // group
	element := buffer.readUInt16() // element

	var name string
	//var name, vm, vr string
	entry, err := LookupDictionary(p.dictionary, Tag{group, element})
	if err != nil {
		if group%2 == 0 {
			name = unknown_group_name
		} else {
			name = private_group_name
		}
	} else {
		name = entry.name
	}

	return &DicomElement{
		Group:   group,
		Element: element,
		Name:    name,
	}

}

// Read x consecutive bytes as a string
func (buffer *dicomBuffer) readString(vl uint32) string {
	chunk := buffer.Next(int(vl))
	chunk = bytes.Trim(chunk, "\x00")   // trim those pesky null bytes
	chunk = bytes.Trim(chunk, "\u200B") // trim zero-width characters
	buffer.p += vl
	return string(chunk)
}

// Read 4 consecutive bytes as a float32
func (buffer *dicomBuffer) readFloat() (val float32) {
	buf := bytes.NewBuffer(buffer.Next(4))
	binary.Read(buf, buffer.bo, &val)
	buffer.p += 4
	return
}

// Read 8 consecutive bytes as a float64
func (buffer *dicomBuffer) readFloat64() (val float64) {
	buf := bytes.NewBuffer(buffer.Next(8))
	binary.Read(buf, buffer.bo, &val)
	buffer.p += 8
	return
}

// Read 2 bytes as a hexadecimal value
//func (buffer *dicomBuffer) readHex() uint16 {
//	return buffer.readUInt16()
//}

// Read 4 bytes as an UInt32
func (buffer *dicomBuffer) readUInt32() (val uint32) {
	buf := bytes.NewBuffer(buffer.Next(4))
	binary.Read(buf, buffer.bo, &val)
	buffer.p += 4
	return
}

// Read 4 bytes as an int32
func (buffer *dicomBuffer) readInt32() (val int32) {
	buf := bytes.NewBuffer(buffer.Next(4))
	binary.Read(buf, buffer.bo, &val)
	buffer.p += 4
	return
}

// Read 2 bytes as an UInt16
func (buffer *dicomBuffer) readUInt16() (val uint16) {
	buf := bytes.NewBuffer(buffer.Next(2))
	binary.Read(buf, buffer.bo, &val)
	buffer.p += 2
	return
}

// Read 2 bytes as an int16
func (buffer *dicomBuffer) readInt16() (val int16) {
	buf := bytes.NewBuffer(buffer.Next(2))
	binary.Read(buf, buffer.bo, &val)
	buffer.p += 2
	return
}

// Read x number of bytes as an array of UInt16 values
func (buffer *dicomBuffer) readUInt16Array(vl uint32) []uint16 {
	slice := make([]uint16, int(vl)/2)

	for i := 0; i < len(slice); i++ {
		slice[i] = buffer.readUInt16()
	}
	buffer.p += vl
	return slice
}

// Read x number of bytes as an array of UInt8 values
func (buffer *dicomBuffer) readUInt8Array(vl uint32) []byte {
	chunk := buffer.Next(int(vl))
	buffer.p += vl
	return chunk
}
