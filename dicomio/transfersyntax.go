package dicomio

import (
	"encoding/binary"
	"fmt"

	"github.com/yasushi-saito/go-dicom/dicomuid"
	"v.io/x/lib/vlog"
)

// Standard list of transfer syntaxes.
var StandardTransferSyntaxes = []string{
	dicomuid.ImplicitVRLittleEndian,
	dicomuid.ExplicitVRLittleEndian,
	dicomuid.ExplicitVRBigEndian,
	dicomuid.DeflatedExplicitVRLittleEndian,
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
	case dicomuid.ImplicitVRLittleEndian:
		fallthrough
	case dicomuid.ExplicitVRLittleEndian:
		fallthrough
	case dicomuid.ExplicitVRBigEndian:
		fallthrough
	case dicomuid.DeflatedExplicitVRLittleEndian:
		return uid, nil
	default:
		e, err := dicomuid.Lookup(uid)
		if err != nil {
			return "", err
		}
		if e.Type != dicomuid.UIDTypeTransferSyntax {
			return "", fmt.Errorf("UID '%s' is not a transfer syntax (is %s)", uid, e.Type)
		}
		// The default is ExplicitVRLittleEndian
		return dicomuid.ExplicitVRLittleEndian, nil
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
	case dicomuid.ImplicitVRLittleEndian:
		return binary.LittleEndian, ImplicitVR, nil
	case dicomuid.DeflatedExplicitVRLittleEndian:
		fallthrough
	case dicomuid.ExplicitVRLittleEndian:
		return binary.LittleEndian, ExplicitVR, nil
	case dicomuid.ExplicitVRBigEndian:
		return binary.BigEndian, ExplicitVR, nil
	default:
		vlog.Fatal(canonical, uid)
		return binary.BigEndian, ExplicitVR, nil
	}
}
