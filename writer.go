package dicom

import (
	"github.com/yasushi-saito/go-dicom/dicomio"
	"encoding/binary"
	"fmt"
	"v.io/x/lib/vlog"
)

// Inverse of ParseFileHeader. Errors are reported via e.Error()
func WriteFileHeader(e *dicomio.Encoder,
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string) {
	e.PushTransferSyntax(binary.LittleEndian, dicomio.ExplicitVR)
	defer e.PopTransferSyntax()
	encodeSingleValue := func(encoder *dicomio.Encoder, tag Tag, v interface{}) {
		elem := DicomElement{
			Tag:   tag,
			Vr:    "", // autodetect
			Vl:    1,
			Value: []interface{}{v},
		}
		EncodeDataElement(encoder, &elem)
	}

	// Encode the meta info first.
	subEncoder := dicomio.NewEncoder(binary.LittleEndian, dicomio.ExplicitVR)
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

	e.WriteZeros(128)
	e.WriteString("DICM")
	encodeSingleValue(e, TagMetaElementGroupLength, uint32(len(metaBytes)))
	e.WriteBytes(metaBytes)
}

// EncodeDataElement encodes one data element. "tag" must be for a scalar
// value. That is, SQ elements are not supported yet. Errors are reported
// through e.Error() and/or E.Finish().
//
// REQUIRES: Each value in values[] must match the VR of the tag. E.g., if tag
// is for UL, then each value must be uint32.
func EncodeDataElement(e *dicomio.Encoder, elem *DicomElement) {
	vr := elem.Vr
	if elem.Vl == UndefinedLength {
		vlog.Fatalf("Encoding undefined-length element not yet supported: %v", elem)
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
		case "SQ":
			sube.SetError(fmt.Errorf("Encoding tag %v not supported yet", vr))
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
	doassert(len(bytes)%2 == 0)
	e.WriteUInt16(elem.Tag.Group)
	e.WriteUInt16(elem.Tag.Element)
	if _, implicit := e.TransferSyntax(); implicit == dicomio.ExplicitVR {
		doassert(len(vr) == 2)
		e.WriteString(vr)
		switch vr {
		case "NA", "OB", "OD", "OF", "OL", "OW", "SQ", "UN", "UC", "UR", "UT":
			e.WriteZeros(2) // two bytes for "future use" (0000H)
			e.WriteUInt32(uint32(len(bytes)))
		default:
			e.WriteUInt16(uint16(len(bytes)))
		}

	} else {
		doassert(implicit == dicomio.ImplicitVR)
		e.WriteUInt32(uint32(len(bytes)))
	}
	e.WriteBytes(bytes)
}
