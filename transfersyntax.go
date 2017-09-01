package dicom

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/glog"
)

// https://www.dicomlibrary.com/dicom/transfer-syntax/

const (
	ImplicitVRLittleEndian         = "1.2.840.10008.1.2"
	ExplicitVRLittleEndian         = "1.2.840.10008.1.2.1"
	ExplicitVRBigEndian            = "1.2.840.10008.1.2.2"
	DeflatedExplicitVRLittleEndian = "1.2.840.10008.1.2.1.99"
)

// Standard list of transfer syntaxes.
var StandardTransferSyntaxes = []string{
	ImplicitVRLittleEndian,
	ExplicitVRLittleEndian,
	ExplicitVRBigEndian,
	DeflatedExplicitVRLittleEndian,
}

// Given an UID that represents a transfer syntax, return the canonical transfer
// syntax UID with the same encoding, from the list StandardTransferSyntaxes.
// Returns an error if the uid is not defined in DICOM standard, or it's not a
// transfer syntax.
//
// TODO(saito) Check the standard to see if we need to accept unknown UIDS as
// explicit little endian.
func CanonicalTransferSyntaxUID(uid string) (string, error) {
	// defaults are explicit VR, little endian
	switch uid {
	case ImplicitVRLittleEndian:
		fallthrough
	case ExplicitVRLittleEndian:
		fallthrough
	case ExplicitVRBigEndian:
		fallthrough
	case DeflatedExplicitVRLittleEndian:
		return uid, nil
	default:
		e, err := LookupUID(uid)
		if err != nil {
			return "", err
		}
		if e.Type != UIDTypeTransferSyntax {
			return "", fmt.Errorf("UID '%s' is not a transfer syntax (is %s)", uid, e.Type)
		}
		// The default is ExplicitVRLittleEndian
		return ExplicitVRLittleEndian, nil
	}
}

// Given a transfer syntax uid, return its encoding.  TrasnferSyntaxUID can be
// any UID that refers to a transfer syntax. It can be, e.g., 1.2.840.10008.1.2
// (it will return LittleEndian, ImplicitVR) or 1.2.840.10008.1.2.4.54 (it will
// return (LittleEndian, ExplicitVR).
func ParseTransferSyntaxUID(uid string) (bo binary.ByteOrder, implicit IsImplicitVR, err error) {
	canonical, err := CanonicalTransferSyntaxUID(uid)
	if err != nil {
		return nil, UnknownVR, err
	}
	switch canonical {
	case ImplicitVRLittleEndian:
		return binary.LittleEndian, ImplicitVR, nil
	case DeflatedExplicitVRLittleEndian:
		fallthrough
	case ExplicitVRLittleEndian:
		return binary.LittleEndian, ExplicitVR, nil
	case ExplicitVRBigEndian:
		return binary.BigEndian, ExplicitVR, nil
	default:
		glog.Fatal(canonical, uid)
		return binary.BigEndian, ExplicitVR, nil
	}
}
