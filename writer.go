package dicom

import (
	"encoding/binary"
	"fmt"
	"github.com/yasushi-saito/go-dicom/dicomio"
)

// Inverse of ParseFileHeader. Errors are reported via e.Error()
func WriteFileHeader(e *dicomio.Encoder,
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string) {
	e.PushTransferSyntax(binary.LittleEndian, dicomio.ExplicitVR)
	defer e.PopTransferSyntax()
	encodeSingleValue := func(encoder *dicomio.Encoder, tag Tag, v interface{}) {
		elem := Element{
			Tag:             tag,
			VR:              "", // autodetect
			UndefinedLength: false,
			Value:           []interface{}{v},
		}
		EncodeDataElement(encoder, &elem)
	}

	// Encode the meta info first.
	subEncoder := dicomio.NewEncoder(binary.LittleEndian, dicomio.ExplicitVR)
	encodeSingleValue(subEncoder, TagFileMetaInformationVersion, []byte("0 1"))
	encodeSingleValue(subEncoder, TagTransferSyntaxUID, transferSyntaxUID)
	encodeSingleValue(subEncoder, TagMediaStorageSOPClassUID, sopClassUID)
	encodeSingleValue(subEncoder, TagMediaStorageSOPInstanceUID, sopInstanceUID)
	encodeSingleValue(subEncoder, TagImplementationClassUID, GoDICOMImplementationClassUID)
	encodeSingleValue(subEncoder, TagImplementationVersionName, GoDICOMImplementationVersionName)
	// TODO(saito) add more
	metaBytes, err := subEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}

	e.WriteZeros(128)
	e.WriteString("DICM")
	encodeSingleValue(e, TagMetaElementGroupLength, uint32(len(metaBytes)))
	e.WriteBytes(metaBytes)
}

func encodeElementHeader(e *dicomio.Encoder, tag Tag, vr string, vl uint32) {
	doassert(vl == undefinedLength || vl%2 == 0)
	e.WriteUInt16(tag.Group)
	e.WriteUInt16(tag.Element)

	_, implicit := e.TransferSyntax()
	if tag.Group == itemSeqGroup {
		implicit = dicomio.ImplicitVR
	}
	if implicit == dicomio.ExplicitVR {
		doassert(len(vr) == 2)
		e.WriteString(vr)
		switch vr {
		case "NA", "OB", "OD", "OF", "OL", "OW", "SQ", "UN", "UC", "UR", "UT":
			e.WriteZeros(2) // two bytes for "future use" (0000H)
			e.WriteUInt32(vl)
		default:
			e.WriteUInt16(uint16(vl))
		}
	} else {
		doassert(implicit == dicomio.ImplicitVR)
		e.WriteUInt32(vl)
	}
}

func writeRawItem(e *dicomio.Encoder, data []byte) {
	encodeElementHeader(e, TagItem, "NA", uint32(len(data)))
	e.WriteBytes(data)
}

func writeBasicOffsetTable(e *dicomio.Encoder, offsets []uint32) {
	byteOrder, _ := e.TransferSyntax()
	subEncoder := dicomio.NewEncoder(byteOrder, dicomio.ImplicitVR)
	for _, offset := range offsets {
		e.WriteUInt32(offset)
	}
	data, err := subEncoder.Finish()
	if err != nil {
		panic(err)
	}
	writeRawItem(e, data)
}

// EncodeDataElement encodes one data element.  Errors are reported through
// e.Error() and/or E.Finish().
//
// REQUIRES: Each value in values[] must match the VR of the tag. E.g., if tag
// is for UL, then each value must be uint32.
func EncodeDataElement(e *dicomio.Encoder, elem *Element) {
	vr := elem.VR
	entry, err := LookupTag(elem.Tag)
	if vr == "" {
		if err == nil {
			vr = entry.VR
		} else {
			vr = "UN"
		}
	} else {
		if err == nil && entry.VR != vr {
			e.SetError(fmt.Errorf("VR value mismatch for tag %s. Element.VR=%v, but tag's VR is %v",
				TagString(elem.Tag), vr, entry.VR))
			return
		}
	}
	doassert(vr != "")
	if elem.Tag == TagPixelData {
		if len(elem.Value) != 1 {
			// TODO(saito) Use of ImageData is a temp hack. Come up with a more proper solution.
			e.SetError(fmt.Errorf("PixelData element must have one value of type ImageData"))
		}
		image, ok := elem.Value[0].(ImageData)
		if !ok {
			e.SetError(fmt.Errorf("PixelData element must have one value of type ImageData"))
		}
		if elem.UndefinedLength {
			encodeElementHeader(e, elem.Tag, vr, undefinedLength)
			writeBasicOffsetTable(e, image.Offsets)
			for _, image := range image.Frames {
				writeRawItem(e, image)
			}
			encodeElementHeader(e, tagItemDelimitationItem, "" /*not used*/, 0)
		} else {
			doassert(len(image.Frames) == 1) // TODO
			encodeElementHeader(e, elem.Tag, vr, uint32(len(image.Frames[0])))
			e.WriteBytes(image.Frames[0])
		}
		return
	}
	if vr == "SQ" {
		if elem.UndefinedLength {
			encodeElementHeader(e, elem.Tag, vr, undefinedLength)
			for _, value := range elem.Value {
				subelem, ok := value.(*Element)
				if !ok || subelem.Tag != TagItem {
					e.SetError(fmt.Errorf("SQ element must be an Item, but found %v", value))
					return
				}
				EncodeDataElement(e, subelem)
			}
			encodeElementHeader(e, tagSequenceDelimitationItem, "" /*not used*/, 0)
		} else {
			sube := dicomio.NewEncoder(e.TransferSyntax())
			for _, value := range elem.Value {
				subelem, ok := value.(*Element)
				if !ok || subelem.Tag != TagItem {
					e.SetError(fmt.Errorf("SQ element must be an Item, but found %v", value))
					return
				}
				EncodeDataElement(sube, subelem)
			}
			bytes, err := sube.Finish()
			if err != nil {
				e.SetError(err)
				return
			}
			encodeElementHeader(e, elem.Tag, vr, uint32(len(bytes)))
			e.WriteBytes(bytes)
		}
	} else if vr == "NA" { // Item
		if elem.UndefinedLength {
			encodeElementHeader(e, elem.Tag, vr, undefinedLength)
			for _, value := range elem.Value {
				subelem, ok := value.(*Element)
				if !ok {
					e.SetError(fmt.Errorf("Item values must be a dicom.Element, but found %v", value))
					return
				}
				EncodeDataElement(e, subelem)
			}
			encodeElementHeader(e, tagItemDelimitationItem, "" /*not used*/, 0)
		} else {
			sube := dicomio.NewEncoder(e.TransferSyntax())
			for _, value := range elem.Value {
				subelem, ok := value.(*Element)
				if !ok {
					e.SetError(fmt.Errorf("Item values must be a dicom.Element, but found %v", value))
					return
				}
				EncodeDataElement(sube, subelem)
			}
			bytes, err := sube.Finish()
			if err != nil {
				e.SetError(err)
				return
			}
			encodeElementHeader(e, elem.Tag, vr, uint32(len(bytes)))
			e.WriteBytes(bytes)
		}
	} else {
		if elem.UndefinedLength {
			e.SetError(fmt.Errorf("Encoding undefined-length element not yet supported: %v", elem))
			return
		}
		sube := dicomio.NewEncoder(e.TransferSyntax())
		for _, value := range elem.Value {
			switch vr {
			case "US":
				sube.WriteUInt16(value.(uint16))
			case "UL":
				sube.WriteUInt32(value.(uint32))
			case "SL":
				sube.WriteInt32(value.(int32))
			case "SS":
				sube.WriteInt16(value.(int16))
			case "FL":
				sube.WriteFloat32(value.(float32))
			case "FD":
				sube.WriteFloat64(value.(float64))
			case "OW":
				fallthrough // TODO(saito) Check that size is even. Byte swap??
			case "OB":
				bytes := value.([]byte)
				sube.WriteBytes(bytes)
				if len(bytes)%2 == 1 {
					sube.WriteByte(0)
				}
			case "AT":
				fallthrough
			case "NA":
				fallthrough
			default:
				s := value.(string)
				sube.WriteString(s)
				if len(s)%2 == 1 {
					sube.WriteByte(0)
				}
			}
		}
		bytes, err := sube.Finish()
		if err != nil {
			e.SetError(err)
			return
		}
		encodeElementHeader(e, elem.Tag, vr, uint32(len(bytes)))
		e.WriteBytes(bytes)
	}
}
