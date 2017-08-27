package dicom

import (
	"encoding/binary"
	"fmt"
	"log"
)

// Inverse of ParseFileHeader. Errors are reported via e.Error()
func WriteFileHeader(e *Encoder,
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string) {
	e.PushTransferSyntax(binary.LittleEndian, ExplicitVR)
	defer e.PopTransferSyntax()
	encodeSingleValue := func(encoder *Encoder, tag Tag, v interface{}) {
		elem := DicomElement{
			Tag:   tag,
			Vr:    "", // autodetect
			Vl:    1,
			Value: []interface{}{v},
		}
		EncodeDataElement(encoder, &elem)
	}

	// Encode the meta info first.
	subEncoder := NewEncoder(binary.LittleEndian, ExplicitVR)
	encodeSingleValue(subEncoder, TagFileMetaInformationVersion, []byte("0 1"))
	encodeSingleValue(subEncoder, TagTransferSyntaxUID, transferSyntaxUID)
	encodeSingleValue(subEncoder, TagMediaStorageSOPClassUID, sopClassUID)
	encodeSingleValue(subEncoder, TagMediaStorageSOPInstanceUID, sopInstanceUID)
	encodeSingleValue(subEncoder, TagImplementationClassUID, DefaultImplementationClassUID)
	encodeSingleValue(subEncoder, TagImplementationVersionName, DefaultImplementationVersionName)
	// TODO(saito) add more
	metaBytes, err := subEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}

	e.EncodeZeros(128)
	e.EncodeString("DICM")
	encodeSingleValue(e, TagMetaElementGroupLength, uint32(len(metaBytes)))
	e.EncodeBytes(metaBytes)
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
		case "AT":
			fallthrough
		case "NA":
			fallthrough
		case "SQ":
			sube.SetError(fmt.Errorf("Encoding tag %v not supported yet", vr))
		default:
			s := value.(string)
			sube.EncodeString(s)
			if len(s)%2 == 1 {
				sube.EncodeByte(0)
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
		doassert(implicit == ImplicitVR)
		e.EncodeUInt32(uint32(len(bytes)))
	}
	e.EncodeBytes(bytes)
}
