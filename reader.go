package dicom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	magic_word = "DICM"

	items_group                 = 0xFFFE
	file_meta_information_group = 0x0002
	unknown_group_name          = "Unknown Group"
	private_group_name          = "Private Data"

	implicit_vr_little_endian = "1.2.840.10008.1.2"
	explicit_vr_little_endian = "1.2.840.10008.1.2.1" // Default Transfer Syntax
	explicit_vr_big_endian    = "1.2.840.10008.1.2.2"
	defaultTransferSyntax     = explicit_vr_little_endian
)

// Errors
var (
	ErrIllegalTag                 = errors.New("Illegal tag found in PixelData")
	ErrTagNotFound                = errors.New("Could not find tag in dicom dictionary")
	ErrBrokenFile                 = errors.New("Invalid DICOM file")
	ErrOddLength                  = errors.New("Encountered odd length Value Length")
	ErrOddLengthI                 = errors.New("Encountered odd length Value Length in Implicit read")
	ErrOddLengthE                 = errors.New("Encountered odd length Value Length in Explicit read")
	ErrUndefLengthNotAllowed      = errors.New("UC, UR and UT may not have an Undefined Length, i.e.,a Value Length of FFFFFFFFH.")
	ErrInvalidValueRepresentation = errors.New("Invalid VR (value representation)")
)

type Reader struct {
	elementNumber uint32
	elementOffset uint32
	absPos        uint32

	r      io.Reader
	parser *Parser

	inPixelData       bool
	inMetaInformation bool

	g, e uint16

	transferSyntaxUID string
	implicit          bool
	bo                binary.ByteOrder
}

// A ParseError is returned for parsing errors.
// The first line is 1.  The first column is 0.
type ParseError struct {
	at                uint32
	element           uint32 // Line where the error occurred
	g, e              uint16
	pos               uint32 // Column (rune index) where the error occurred
	err               error  // The actual error
	transferSyntaxUID string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("at %d Element %d (%04X,%04X), Pos %d, TransferSyntaxUID: %s, Error: %s", e.at, e.element, e.g, e.e, e.pos, e.transferSyntaxUID, e.err)
}

// error creates a new ParseError based on err.
func (r *Reader) error(err error) error {
	return &ParseError{
		at:                r.elementOffset,
		element:           r.elementNumber,
		e:                 r.g,
		g:                 r.e,
		pos:               r.absPos - r.elementOffset,
		err:               err,
		transferSyntaxUID: r.transferSyntaxUID,
	}
}

// These are the errors that can be returned in ParseError.Error
var (
	ErrTrailingComma = errors.New("extra delimiter at end of line") // no longer used
	ErrBareQuote     = errors.New("bare \" in non-quoted-field")
	ErrQuote         = errors.New("extraneous \" in field")
	ErrFieldCount    = errors.New("wrong number of fields in line")
)

// NewReader returns a new Reader that reads from r.
func NewReader(r io.Reader) *Reader {

	parser, err := NewParser()
	if err != nil {
		panic(err)
	}

	return &Reader{
		r:      r,
		parser: parser,
	}
}

func (r *Reader) readPreamble() (err error) {

	r.readUInt8Array(128) // skip first 128 bytes

	// check for magic word
	magicWord := r.readString(4)

	if magicWord != magic_word {
		return r.error(ErrBrokenFile)
	}
	return err
}

// Read bytes from the underlying Reader
func (r *Reader) readBytes(n uint32) []byte {

	buf := make([]byte, n)

	i, err := r.r.Read(buf)
	if err != nil {
		if err == io.EOF {
			panic(err)
		} else {
			panic(r.error(err))
		}
	}
	r.absPos += uint32(i)

	return buf
}

func (r *Reader) decodeValueLength(vr string, explicit bool) (uint32, bool, error) {

	var err error
	var vl uint32
	ulen := false

	if explicit {

		// long value representations
		switch vr {
		case "NA", "OB", "OD", "OF", "OL", "OW", "SQ", "UN", "UC", "UR", "UT":
			r.readUInt8Array(2) // ignore two bytes for "future use" (0000H)

			vl = r.readUInt32()
			// Rectify Undefined Length VL
			if vl == 0xffffffff {
				switch vr {
				case "UC", "UR", "UT":
					return 0, ulen, ErrUndefLengthNotAllowed
				default:
					ulen = true
					vl = 0
				}
			}
		default:
			vl = uint32(r.readUInt16())
			// Rectify Undefined Length VL
			if vl == 0xffff {
				ulen = true
				vl = 0
			}
		}
	} else {

		vl = r.readUInt32()
		// Rectify Undefined Length VL
		if vl == 0xffffffff {
			ulen = true
			vl = 0
		}

	}

	// Error when encountering odd length
	if vl > 0 && vl%2 != 0 {
		return 0, ulen, ErrOddLength
	}

	return vl, ulen, err
}

// Read the VR from the DICOM ditionary
// The VL is a 32-bit unsigned integer
func (r *Reader) readImplicit(elem *DicomElement) (string, uint32) {

	var vr string

	entry, err := r.parser.getDictEntry(elem.group, elem.element)
	if err != nil {
		vr = "UN"
	} else {
		vr = entry.vr
	}

	vl, ulen, err := r.decodeValueLength(vr, false)
	elem.undefLen = ulen
	if err == ErrOddLength {
		panic(r.error(ErrOddLengthI))
	}

	return vr, vl
}

// The VR is represented by the next two consecutive bytes
// The VL depends on the VR value
func (r *Reader) readExplicit(elem *DicomElement) (string, uint32) {

	vr := r.readString(2)

	vl, ulen, err := r.decodeValueLength(vr, true)
	elem.undefLen = ulen

	if err == ErrOddLength {
		panic(r.error(ErrOddLengthE))
	}

	return vr, vl
}

func (r *Reader) checkVR(vr string) string {
	switch vr {
	case "AE", "AS", "AT", "CS", "DA", "DS", "DT", "FL", "FD", "IS", "LO", "LT", "OB", "OD", "OF", "OL", "OW", "PN", "SH", "SL", "SQ", "SS", "ST", "TM", "UC", "UI", "UL", "UN", "UR", "US", "UT", "NA":
		return vr
	default:
		//panic(r.error(ErrInvalidValueRepresentation))
		return vr
	}
}

func (r *Reader) setTransferSyntax(ts string) {
	switch ts {
	case implicit_vr_little_endian:
		r.implicit = true
		r.bo = binary.LittleEndian
	case explicit_vr_big_endian:
		r.implicit = false
		r.bo = binary.BigEndian
	default:
		// explicit_vr_little_endian = "1.2.840.10008.1.2.1"  Default Transfer Syntax
		// 	Compressed pixel data transfer syntax are always explicit VR little Endian
		//	(so you can call JPEG baseline 1.2.840.10008.1.2.4.50 for example "explicit little endian jpeg baseline")
		r.implicit = false
		r.bo = binary.LittleEndian
	}
}

// Read reads and parses a single dicomElement
func (r *Reader) Read() (dicomElement *DicomElement, err error) {

	defer func() {
		if rec := recover(); rec != nil {
			switch x := rec.(type) {
			case error:
				err = x
			}
			dicomElement = nil
		}
	}()

	r.g, r.e = 0, 0 // Info needed by ParseError, new element initialization

	if r.absPos == 0 {
		//File Meta Information shall be encoded using the Explicit VR Little Endian Transfer Syntax (UID=1.2.840.10008.1.2.1)
		r.inMetaInformation = true
		r.setTransferSyntax(defaultTransferSyntax)
		err = r.readPreamble()
	}

	r.elementNumber++
	r.elementOffset = r.absPos
	elem := r.readTag()

	var vr string     // Value Representation
	var vl uint32 = 0 // Value Length

	if elem.Name == "PixelData" {
		r.inPixelData = true
	}

	if r.inMetaInformation {
		if elem.group > file_meta_information_group {
			// set Tranfer Syntax
			r.inMetaInformation = false
			r.setTransferSyntax(r.transferSyntaxUID)
		}
	}

	implicit := r.implicit
	// The elements for group 0xFFFE should be Encoded as Implicit VR.
	// PS 3.5 - Section 7.5: "Nesting of Data Sets"
	if elem.group == items_group {
		implicit = true
	}

	if implicit {
		vr, vl = r.readImplicit(elem)
	} else {
		vr, vl = r.readExplicit(elem)
		vr = r.checkVR(vr)
	}

	elem.vr = vr
	elem.vl = vl

	// data
	var data elementValues
	uvl := vl
	valLen := uint32(vl)

	for uvl > 0 {
		switch vr {
		case "AT":
			valLen = 2
			dcmVal := r.readHex()
			data = append(data, dcmVal)
		//TODO:  DA, DT, TM
		// implement Range Matching and Specific Character Set (0008,0005) see PS3.4 C.2.2.2
		case "DA":
			valLen = vl
			dcmVal := &DcmDA{}
			dcmVal.readData(r.readBytes(vl))
			data = append(data, dcmVal)
		case "TM":
			valLen = vl
			dcmVal := &DcmTM{}
			dcmVal.readData(r.readBytes(vl))
			data = append(data, dcmVal)
		case "DT":
			dcmVal := &DcmDT{}
			dcmVal.readData(r.readBytes(vl))
			data = append(data, dcmVal)
		case "DS":
			valLen = vl
			dcmVal := &DcmDS{}
			dcmVal.readData(r.readBytes(vl))
			data = append(data, dcmVal)
		case "UL":
			valLen = 4
			dcmVal := &DcmUL{}
			dcmVal.readData(r.readUInt32())
			data = append(data, dcmVal)
		case "SL":
			valLen = 4
			data = append(data, r.readInt32())
		case "US":
			valLen = 2
			data = append(data, r.readUInt16())
		case "SS":
			valLen = 2
			data = append(data, r.readInt16())
		case "FL":
			valLen = 4
			data = append(data, r.readFloat())
		case "FD":
			valLen = 8
			data = append(data, r.readFloat64())
		case "OW":
			valLen = vl
			data = append(data, r.readUInt16Array(vl))
		case "OB":
			valLen = vl
			data = append(data, r.readUInt8Array(vl))
		case "NA":
			valLen = vl
			if elem.Name == "Item" && r.inPixelData {
				data = append(data, r.readUInt8Array(vl))
			}
		//case "XS": ??

		case "SQ":
			valLen = vl
			data = append(data, "")

		default:
			valLen = vl
			str := strings.TrimRight(r.readString(vl), " ")
			strs := strings.Split(str, "\\")
			for _, s := range strs {
				data = append(data, s)
			}
		}
		uvl -= valLen
	}

	if r.inMetaInformation {
		if elem.group == file_meta_information_group {
			// Process File Meta Information
			if elem.Name == "TransferSyntaxUID" {
				r.transferSyntaxUID = data[0].(string)
			}
		}
	}

	elem.p = r.elementOffset
	elem.Values = data
	elem.elemLen = r.absPos - r.elementOffset

	return elem, err
}

// ReadAll reads all the remaining dicomElements from r.
// A successful call returns err == nil, not err == io.EOF. Because ReadAll is
// defined to read until EOF, it does not treat end of file as an error to be
// reported.
func (r *Reader) ReadAll() (des []DicomElement, err error) {
	for {
		de, err := r.Read()
		if err == io.EOF {
			return des, nil
		}
		if err != nil {
			return des, err
		}
		des = append(des, *de)
	}
}

// Read a DICOM data element's tag value
// ie. (0002,0000)
// added  Value Multiplicity PS 3.5 6.4
func (r *Reader) readTag() (de *DicomElement) {
	group := r.readHex()   // group
	element := r.readHex() // element

	r.g = group
	r.e = element

	var name string
	//var name, vm, vr string
	entry, err := r.parser.getDictEntry(group, element)
	if err != nil {
		if group%2 == 0 {
			name = unknown_group_name
		} else {
			name = private_group_name
		}
	} else {
		name = entry.name
	}

	de = &DicomElement{
		group:   group,
		element: element,
		Name:    name,
	}
	return de

}

// Read x consecutive bytes as a string
func (r *Reader) readString(vl uint32) string {
	chunk := make([]byte, vl)
	buf := bytes.NewBuffer(r.readBytes(vl))
	binary.Read(buf, r.bo, &chunk)
	chunk = bytes.Trim(chunk, "\x00")   // trim those pesky null bytes
	chunk = bytes.Trim(chunk, "\u200B") // trim zero-width characters
	return string(chunk)
}

// Read 4 consecutive bytes as a float32
func (r *Reader) readFloat() (val float32) {

	buf := bytes.NewBuffer(r.readBytes(4))
	binary.Read(buf, r.bo, &val)
	return val
}

// Read 8 consecutive bytes as a float64
func (r *Reader) readFloat64() (val float64) {

	buf := bytes.NewBuffer(r.readBytes(8))
	binary.Read(buf, r.bo, &val)
	return val
}

// Read 2 bytes as a hexadecimal value
func (r *Reader) readHex() (val uint16) {
	val = r.readUInt16()
	return val
}

// Read 4 bytes as an UInt32
func (r *Reader) readUInt32() (val uint32) {

	buf := bytes.NewBuffer(r.readBytes(4))
	binary.Read(buf, r.bo, &val)
	return val
}

// Read 4 bytes as an int32
func (r *Reader) readInt32() (val int32) {

	buf := bytes.NewBuffer(r.readBytes(4))
	binary.Read(buf, r.bo, &val)
	return val
}

// Read 2 bytes as an UInt16
func (r *Reader) readUInt16() (val uint16) {

	buf := bytes.NewBuffer(r.readBytes(2))
	binary.Read(buf, r.bo, &val)
	return val

}

// Read 2 bytes as an int16
func (r *Reader) readInt16() (val int16) {

	buf := bytes.NewBuffer(r.readBytes(2))
	binary.Read(buf, r.bo, &val)
	return val
}

// Read x number of bytes as an array of UInt16 values
func (r *Reader) readUInt16Array(vl uint32) (slice []uint16) {

	var val uint16
	slice = make([]uint16, int(vl)/2)
	for i := 0; i < len(slice); i++ {
		buf := bytes.NewBuffer(r.readBytes(2))
		binary.Read(buf, r.bo, &val)
		slice[i] = val
	}
	return slice
}

// Read x number of bytes as an array of UInt8 values
func (r *Reader) readUInt8Array(vl uint32) []byte {

	chunk := make([]byte, vl)
	buf := bytes.NewBuffer(r.readBytes(vl))
	binary.Read(buf, r.bo, &chunk)
	return chunk

}
