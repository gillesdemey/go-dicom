package dicom

// Dictionary supports looking up DICOM data dictionary as defined in
//
// ftp://medical.nema.org/medical/dicom/2011/11_06pu.pdf

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

// Tag is a <group, element> tuple that identifies an element type in a DICOM
// file. List of standard tags are defined in tagdict.go. See also:
//
// ftp://medical.nema.org/medical/dicom/2011/11_06pu.pdf
type Tag struct {
	// group and element are results of parsing the hex-pair tag, such as (1000,10008)
	Group   uint16
	Element uint16
}

func (t *Tag) String() string {
	return fmt.Sprintf("(%04x,%04x)", t.Group, t.Element)
}

type TagDictEntry struct {
	Tag Tag

	// Data encoding "UL", "CS", etc.
	VR string
	// Human-readable name of the tag
	Name string
	// Cardinality.
	VM      string
	Version string
}

var (
	tagItem                     Tag
	tagItemDelimitationItem     Tag
	tagSequenceDelimitationItem Tag

	// Standard file metadata tags, with group=2
	TagMetaElementGroupLength         Tag // Always the first element in a file
	TagFileMetaInformationGroupLength Tag
	TagFileMetaInformationVersion     Tag
	TagMediaStorageSOPClassUID        Tag
	TagMediaStorageSOPInstanceUID     Tag
	TagImplementationClassUID         Tag
	TagImplementationVersionName      Tag
	TagTransferSyntaxUID              Tag

	TagPixelData Tag
)

// (group, element) -> tag information
type tagDict map[Tag]TagDictEntry

var singletonDict tagDict

// Create a new, fully filled dictionary.
func init() {
	reader := csv.NewReader(bytes.NewReader([]byte(tagDictData)))
	reader.Comma = '\t'  // tab separated file
	reader.Comment = '#' // comments start with #
	singletonDict = make(tagDict)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		tag, err := splitTag(row[0])
		if err != nil {
			continue // we don't support groups yet
		}
		singletonDict[tag] = TagDictEntry{
			Tag:     tag,
			VR:      strings.ToUpper(row[1]),
			Name:    row[2],
			VM:      row[3],
			Version: row[4],
		}
	}
	tagItem = MustLookupTag(Tag{0xfffe, 0xe000}).Tag
	tagItemDelimitationItem = MustLookupTag(Tag{0xfffe, 0xe00d}).Tag
	tagSequenceDelimitationItem = MustLookupTag(Tag{0xfffe, 0xe0dd}).Tag
	TagMetaElementGroupLength = MustLookupTag(Tag{2, 0}).Tag

	TagFileMetaInformationGroupLength = MustLookupTag(Tag{2, 0}).Tag
	TagFileMetaInformationVersion = MustLookupTag(Tag{2, 1}).Tag
	TagMediaStorageSOPClassUID = MustLookupTag(Tag{2, 2}).Tag
	TagMediaStorageSOPInstanceUID = MustLookupTag(Tag{2, 3}).Tag
	TagTransferSyntaxUID = MustLookupTag(Tag{2, 0x10}).Tag
	TagImplementationClassUID = MustLookupTag(Tag{2, 0x12}).Tag
	TagImplementationVersionName = MustLookupTag(Tag{2, 0x13}).Tag

	TagPixelData = MustLookupTag(Tag{0x7fe0, 0x0010}).Tag
}

// LookupTag finds information about the given tag. If the tag is undefined or
// is retired in the standard, it returns an error.
//
// Example: LookupTagByName(Tag{2,0x10})
func LookupTag(tag Tag) (TagDictEntry, error) {
	entry, ok := singletonDict[tag]
	if !ok {
		// (0000-u-ffff,0000)	UL	GenericGroupLength	1	GENERIC
		if tag.Group%2 == 0 && tag.Element == 0x0000 {
			entry = TagDictEntry{tag, "UL", "GenericGroupLength", "1", "GENERIC"}
		} else {
			return TagDictEntry{}, fmt.Errorf("Could not find tag (0x%x, 0x%x) in dictionary", tag.Group, tag.Element)
		}
	}
	return entry, nil
}

// LookupTag finds information about the tag with the given name. If the tag is undefined or
// is retired in the standard, it returns an error.
//
// Example: LookupTagByName("TransferSyntaxUID")
func LookupTagByName(name string) (TagDictEntry, error) {
	for _, ent := range singletonDict {
		if ent.Name == name {
			return ent, nil
		}
	}
	return TagDictEntry{}, fmt.Errorf("Could not find tag with name %s", name)
}

// Like LookupTag, but panics on error.
func MustLookupTag(tag Tag) TagDictEntry {
	e, err := LookupTag(tag)
	if err != nil {
		log.Panicf("tag %s not found: %s", tag, err)
	}
	return e
}

// TagDebugString returns a human-readable diagnostic string for the tag
func TagDebugString(tag Tag) string {
	e, err := LookupTag(tag)
	if err != nil {
		return fmt.Sprintf("(%04x,%04x)[??]", tag.Group, tag.Element)
	}
	return fmt.Sprintf("(%04x,%04x)[%s]", tag.Group, tag.Element, e.Name)
}

// Split a tag into a group and element, represented as a hex value
// TODO: support group ranges (6000-60FF,0803)
func splitTag(tag string) (Tag, error) {
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
