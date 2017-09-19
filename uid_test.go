package dicom_test

import (
	"testing"

	"github.com/yasushi-saito/go-dicom"
)

func TestStandardUIDs(t *testing.T) {
	if dicom.PatientRootQRFind != "1.2.840.10008.5.1.4.1.2.1.1" {
		t.Error(dicom.PatientRootQRFind)
	}
}

func TestLookupUID(t *testing.T) {
	u := dicom.MustLookupUID("1.2.840.10008.15.0.4.8")
	if u.Name != "dicomTransferCapability" {
		t.Error(u)
	}
	if u.Type != "LDAP OID" {
		t.Error(u)
	}
}
