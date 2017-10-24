package dicom

//go:generate ./generate_dimse_messages.py

// Standard DICOM tag definitions.
//
// ftp://medical.nema.org/medical/dicom/2011/11_06pu.pdf

import (
	"fmt"
	"strconv"
	"strings"
	"v.io/x/lib/vlog"
)

// Tag is a <group, element> tuple that identifies an element type in a DICOM
// file. List of standard tags are defined in tag.go. See also:
//
// ftp://medical.nema.org/medical/dicom/2011/11_06pu.pdf
type Tag struct {
	// Group and element are results of parsing the hex-pair tag, such as (1000,10008)
	Group   uint16
	Element uint16
}

// Return a string of form "(0008,1234)", where 0x0008 is t.Group,
// 0x1234 is t.Element.
func (t *Tag) String() string {
	return fmt.Sprintf("(%04x,%04x)", t.Group, t.Element)
}

type TagInfo struct {
	Tag Tag
	// Data encoding "UL", "CS", etc.
	VR string
	// Human-readable name of the tag, e.g., "CommandDataSetType"
	Name string
	// Cardinality (# of values expected in the element)
	VM string
}

const TagMetadataGroup = 2

// VRKind defines the golang encoding of a VR.
type VRKind int

const (
	// Element stores a list of strings
	VRStringList VRKind = iota
	// Element stores a []byte
	VRBytes
	// Element stores a string
	VRString
	// Element stores a list of uint16s
	VRUInt16List
	// Element stores a list of uint32s
	VRUInt32List
	// Element stores a list of int16s
	VRInt16List
	// Element stores a list of int32s
	VRInt32List
	// Element stores a list of float32s
	VRFloat32List
	// Element stores a list of float64s
	VRFloat64List
	// Element stores a list of *Elements, w/ TagItem
	VRSequence
	// Element stores a list of *Elements
	VRItem
	// Element stores a list of Tags
	VRTagList
	// Element stores a date string. Use ParseDate() to parse the date string.
	VRDate
	// Element stores a PixelDataInfo
	VRPixelData
)

// GetVRKind returns the golang value encoding of an element with <tag, vr>.
func GetVRKind(tag Tag, vr string) VRKind {
	if tag == TagItem {
		return VRItem
	} else if tag == TagPixelData {
		return VRPixelData
	}
	switch vr {
	case "DA":
		return VRDate
	case "AT":
		return VRTagList
	case "OW", "OB":
		return VRBytes
	case "LT", "UT":
		return VRString
	case "UL":
		return VRUInt32List
	case "SL":
		return VRInt32List
	case "US":
		return VRUInt16List
	case "SS":
		return VRInt16List
	case "FL":
		return VRFloat32List
	case "FD":
		return VRFloat64List
	case "SQ":
		return VRSequence
	default:
		return VRStringList
	}
}

// FindTag finds information about the given tag. If the tag is not part of
// the DICOM standard, or is retired from the standard, it returns an error.
func FindTag(tag Tag) (TagInfo, error) {
	maybeInitTagDict()
	entry, ok := tagDict[tag]
	if !ok {
		// (0000-u-ffff,0000)	UL	GenericGroupLength	1	GENERIC
		if tag.Group%2 == 0 && tag.Element == 0x0000 {
			entry = TagInfo{tag, "UL", "GenericGroupLength", "1"}
		} else {
			return TagInfo{}, fmt.Errorf("Could not find tag (0x%x, 0x%x) in dictionary", tag.Group, tag.Element)
		}
	}
	return entry, nil
}

// Like FindTag, but panics on error.
func MustFindTag(tag Tag) TagInfo {
	e, err := FindTag(tag)
	if err != nil {
		vlog.Fatalf("tag %v not found: %s", tag, err)
	}
	return e
}

// FindTag finds information about the tag with the given name. If the tag is not part of
// the DICOM standard, or is retired from the standard, it returns an error.
//
//   Example: FindTagByName("TransferSyntaxUID")
func FindTagByName(name string) (TagInfo, error) {
	maybeInitTagDict()
	for _, ent := range tagDict {
		if ent.Name == name {
			return ent, nil
		}
	}
	return TagInfo{}, fmt.Errorf("Could not find tag with name %s", name)
}

// TagString returns a human-readable diagnostic string for the tag
func TagString(tag Tag) string {
	e, err := FindTag(tag)
	if err != nil {
		return fmt.Sprintf("(%04x,%04x)[??]", tag.Group, tag.Element)
	}
	return fmt.Sprintf("(%04x,%04x)[%s]", tag.Group, tag.Element, e.Name)
}

// Split a tag into a group and element, represented as a hex value
// TODO: support group ranges (6000-60FF,0803)
func parseTag(tag string) (Tag, error) {
	parts := strings.Split(strings.Trim(tag, "()"), ",")
	group, err := strconv.ParseInt(parts[0], 16, 0)
	if err != nil {
		return Tag{}, err
	}
	elem, err := strconv.ParseInt(parts[1], 16, 0)
	if err != nil {
		return Tag{}, err
	}
	return Tag{Group: uint16(group), Element: uint16(elem)}, nil
}
