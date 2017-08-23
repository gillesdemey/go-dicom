package dicom

import (
	//"log"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type DicomFile struct {
	Elements []DicomElement
}

// Errors
var (
	ErrIllegalTag            = errors.New("Illegal tag found in PixelData")
	ErrBrokenFile            = errors.New("Invalid DICOM file")
	ErrOddLength             = errors.New("Encountered odd length Value Length")
	ErrUndefLengthNotAllowed = errors.New("UC, UR and UT may not have an Undefined Length, i.e.,a Value Length of FFFFFFFFH.")
)

const (
	magicWord = "DICM"
)

// Parse a byte array, returns a DICOM file struct
func (p *Parser) Parse(in io.Reader, bytes int64) (*DicomFile, error) {
	// buffer := newDicomBuffer(buff) //*di.Bytes)
	buffer := NewDecoder(in,
		bytes,
		binary.LittleEndian,
		false)
	buffer.Skip(128) // skip preamble

	// check for magic word
	if s := buffer.DecodeString(4); s != magicWord {
		return nil, ErrBrokenFile
	}
	file := &DicomFile{}

	// (0002,0000) MetaElementGroupLength
	metaElem := ReadDataElement(buffer)
	if buffer.Error() != nil {
		return nil, buffer.Error()
	}
	if len(metaElem.Value) < 1 {
		return nil, fmt.Errorf("No value found in meta element")
	}
	metaLength, ok := metaElem.Value[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("Expect integer as metaElem.Values[0], but found '%v'", metaElem.Value[0])
	}
	if buffer.Len() <= 0 {
		return nil, fmt.Errorf("No data element found")
	}
	appendDataElement(file, metaElem)

	// Read meta tags
	start := buffer.Len()
	prevLen := buffer.Len()
	for start-buffer.Len() < int64(metaLength) && buffer.Error() == nil {
		elem := ReadDataElement(buffer)
		appendDataElement(file, elem)
		if buffer.Len() >= prevLen {
			panic("Failed to consume buffer")
		}
		prevLen = buffer.Len()
	}
	if buffer.Error() != nil {
		return nil, buffer.Error()
	}
	// read endianness and explicit VR
	endianess, implicit, err := file.getTransferSyntax()
	if err != nil {
		return nil, err
	}

	// modify buffer according to new TransferSyntaxUID
	buffer.bo = endianess
	buffer.implicit = implicit

	for buffer.Len() != 0 && buffer.Error() == nil {
		elem := ReadDataElement(buffer)
		if buffer.Error() != nil {
			break
		}
		appendDataElement(file, elem)
		// if elem.Vr == "SQ" {
		// 	_, err = p.readItems(file, buffer, elem)
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// }
		// if elem.Name == "PixelData" {
		// 	err = p.readPixelItems(file, buffer, elem)
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// 	break
		// }

	}
	return file, buffer.Error()
}

func doassert(x bool) {
	if !x {
		panic("doassert")
	}
}

// func (p *Parser) readItems(file *DicomFile, buffer *Decoder, sq *DicomElement) (uint32, error) {
// 	sq.IndentLevel++
// 	sqLength := sq.Vl
// 	elem, err := ReadDataElement(buffer)
// 	if err != nil {
// 		return 0, err
// 	}
// 	log.Printf("READING ELEM: %v %v", sq, elem)
// 	// The SQ element must contain one Item element.
// 	if elem.Tag != tagItem.Tag {
// 		return 0, fmt.Errorf("Expect an Item element for SQ %v, but found %v",sq, elem)
// 	}
// 	elem.IndentLevel = sq.IndentLevel

// 	sqAcum := elem.elemLen
// 	itemLength := elem.Vl
// 	itemAcum := uint32(0)

// 	if elem.Vl == UndefinedLength {
// 		for buffer.Len() != 0 && buffer.Error() == nil {
// 			if elem.Tag == tagSequenceDelimitationItem.Tag {
// 				break
// 			}
// 			elem, err = ReadDataElement(buffer)
// 			appendDataElement(file, elem)
// 			if err != nil {
// 				return 0, err
// 			}
// 			elem.IndentLevel = sq.IndentLevel

// 		}
// 	} else if elem.Vl > 0 {
// 		for buffer.Len() != 0 {
// 			appendDataElement(file, elem)

// 			if elem.Vr == "SQ" {
// 				l, _ := p.readItems(file, buffer, elem)
// 				sqAcum += l
// 			}

// 			if itemAcum == itemLength {
// 				break
// 			}

// 			if sqAcum == sqLength {
// 				break
// 			}

// 			elem, err = ReadDataElement(buffer)
// 			if err != nil {
// 				return 0, err
// 			}
// 			elem.IndentLevel = sq.IndentLevel
// 			if elem.Name == "Item" {
// 				itemLength = elem.Vl
// 			}
// 			itemAcum += elem.elemLen
// 			sqAcum += elem.elemLen

// 		}
// 	} else {
// 		// ITEM 0 LEN
// 	}
// 	return sqAcum, nil
// }

func (p *Parser) readPixelItems(file *DicomFile, buffer *Decoder, sq *DicomElement) error {
	elem := ReadDataElement(buffer)
	if buffer.Error() != nil {
		return buffer.Error()
	}
	for buffer.Len() != 0 && buffer.Error() == nil {
		if elem.Name == "Item" {
			elem.Value = append(elem.Value, buffer.DecodeBytes(int(elem.Vl)))
		}
		appendDataElement(file, elem)
		elem = ReadDataElement(buffer)
	}
	appendDataElement(file, elem)
	return buffer.Error()
}

// Append a dataElement to the DicomFile
func appendDataElement(file *DicomFile, elem *DicomElement) {
	file.Elements = append(file.Elements, *elem)

}

// Finds the SyntaxTrasnferUID and returns the endianess and implicit VR for the file
func (file *DicomFile) getTransferSyntax() (binary.ByteOrder, bool, error) {

	var err error

	elem, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		return nil, true, err
	}

	ts := elem.Value[0].(string)

	// defaults are explicit VR, little endian
	switch ts {
	case ImplicitVRLittleEndian:
		return binary.LittleEndian, true, nil
	case ExplicitVRLittleEndian:
		return binary.LittleEndian, false, nil
	case ExplicitVRBigEndian:
		return binary.BigEndian, false, nil
	default:
		// panic(fmt.Sprintf("Unknown transfer syntax: %s", ts)) // TODO
		return binary.LittleEndian, false, nil
	}

}

// Lookup a tag by name
func (file *DicomFile) LookupElement(name string) (*DicomElement, error) {

	for _, elem := range file.Elements {
		if elem.Name == name {
			return &elem, nil
		}
	}
	return nil, fmt.Errorf("Could not find element '%s' in dicom file", name)
}
