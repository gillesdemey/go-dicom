package dicom

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
