package dicom

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type DicomFile struct {
	Elements []DicomElement
}

// Errors
var (
	ErrIllegalTag            = errors.New("Illegal tag found in PixelData")
	ErrTagNotFound           = errors.New("Could not find tag in dicom dictionary")
	ErrBrokenFile            = errors.New("Invalid DICOM file")
	ErrOddLength             = errors.New("Encountered odd length Value Length")
	ErrUndefLengthNotAllowed = errors.New("UC, UR and UT may not have an Undefined Length, i.e.,a Value Length of FFFFFFFFH.")
)

const (
	magic_word                = "DICM"
	implicit_vr_little_endian = "1.2.840.10008.1.2"
	explicit_vr_little_endian = "1.2.840.10008.1.2.1"
	explicit_vr_big_endian    = "1.2.840.10008.1.2.2"
)

// Parse a byte array, returns a DICOM file struct
func (p *Parser) Parse(buff []byte) (*DicomFile, error) {
	buffer := newDicomBuffer(buff) //*di.Bytes)

	buffer.Next(128) // skip preamble
	buffer.p = +128

	// check for magic word
	if magicWord := string(buffer.Next(4)); magicWord != magic_word {
		return nil, ErrBrokenFile
	}
	file := &DicomFile{}

	// (0002,0000) MetaElementGroupLength
	metaElem, err := buffer.readDataElement(p)
	if err != nil {
		return nil, err
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
	p.appendDataElement(file, metaElem)

	// Read meta tags
	start := buffer.Len()
	prevLen := buffer.Len()
	for start-buffer.Len() < int(metaLength) {
		elem, err := buffer.readDataElement(p)
		if err != nil {
			return nil, err
		}
		p.appendDataElement(file, elem)
		if buffer.Len() >= prevLen {
			panic("Failed to consume buffer")
		}
		prevLen = buffer.Len()
	}

	// read endianness and explicit VR
	endianess, implicit, err := file.getTransferSyntax()
	if err != nil {
		return nil, err
	}

	// modify buffer according to new TransferSyntaxUID
	buffer.bo = endianess
	buffer.implicit = implicit

	for buffer.Len() != 0 {
		elem, err := buffer.readDataElement(p)
		if err != nil {
			return nil, err
		}
		p.appendDataElement(file, elem)

		if elem.Vr == "SQ" {
			_, err = p.readItems(file, buffer, elem)
			if err != nil {
				return nil, err
			}
		}

		if elem.Name == "PixelData" {
			err = p.readPixelItems(file, buffer, elem)
			if err != nil {
				return nil, err
			}
			break
		}

	}

	return file, nil
}

func (p *Parser) readItems(file *DicomFile, buffer *dicomBuffer, sq *DicomElement) (uint32, error) {
	sq.IndentLevel++
	sqLength := sq.Vl

	if sqLength == 0 {
		return 0, nil
	}

	elem, err := buffer.readDataElement(p)
	if err != nil {
		return 0, err
	}

	elem.IndentLevel = sq.IndentLevel

	sqAcum := elem.elemLen
	itemLength := elem.Vl
	itemAcum := uint32(0)

	if elem.Name == "Item" {
		if elem.Vl > 0 {
			for buffer.Len() != 0 {
				p.appendDataElement(file, elem)

				if elem.Vr == "SQ" {
					l, _ := p.readItems(file, buffer, elem)
					sqAcum += l
				}

				if itemAcum == itemLength {
					break
				}

				if sqAcum == sqLength {
					break
				}

				elem, err = buffer.readDataElement(p)
				if err != nil {
					return 0, err
				}
				elem.IndentLevel = sq.IndentLevel
				if elem.Name == "Item" {
					itemLength = elem.Vl
				}
				itemAcum += elem.elemLen
				sqAcum += elem.elemLen

			}

		} else if elem.undefLen == true {
			//log.Println("____ ITEM UNDEF LEN ____")
			for buffer.Len() != 0 {

				if elem.Vr == "SQ" {
					p.readItems(file, buffer, elem)
				}

				if elem.Name == "SequenceDelimitationItem" {
					break
				}

				p.appendDataElement(file, elem)
				elem, err = buffer.readDataElement(p)
				if err != nil {
					return 0, err
				}
				elem.IndentLevel = sq.IndentLevel

			}
		} else {
			// ITEM 0 LEN
		}
	}

	return sqAcum, nil

}

func (p *Parser) readPixelItems(file *DicomFile, buffer *dicomBuffer, sq *DicomElement) error {
	elem, err := buffer.readDataElement(p)
	if err != nil {
		return err
	}
	for buffer.Len() != 0 {
		if elem.Name == "Item" {
			elem.Value = append(elem.Value, buffer.readUInt8Array(elem.Vl))
		}
		p.appendDataElement(file, elem)
		elem, err = buffer.readDataElement(p)
		if err != nil {
			return err
		}

	}
	p.appendDataElement(file, elem)
	return nil
}

// Append a dataElement to the DicomFile
func (p *Parser) appendDataElement(file *DicomFile, elem *DicomElement) {
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
	case implicit_vr_little_endian:
		return binary.LittleEndian, true, nil
	case explicit_vr_little_endian:
		return binary.LittleEndian, false, nil
	case explicit_vr_big_endian:
		return binary.BigEndian, false, nil
	}

	return binary.LittleEndian, false, nil

}

// Lookup a tag by name
func (file *DicomFile) LookupElement(name string) (*DicomElement, error) {

	for _, elem := range file.Elements {
		if elem.Name == name {
			return &elem, nil
		}
	}

	return nil, ErrTagNotFound
}
