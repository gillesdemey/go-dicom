package dicom

// Dictionary supports looking up DICOM data dictionary as defined in
//
// ftp://medical.nema.org/medical/dicom/2011/11_06pu.pdf

import (
	"fmt"
	"bytes"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

type Tag struct {
	// group and element are results of parsing the hex-pair tag, such as (1000,10008)
	Group uint16
	Element uint16
}

type TagDictEntry struct {
	Tag Tag

	// Data encoding
	VR string
	// Human-readable name of the tag
	Name    string
	VM      string
	Version string
}


// Combination of group and element.
type tagDictKey uint32

func makeTagDictKey(tag Tag) tagDictKey {
	return (tagDictKey(tag.Group) << 16) | tagDictKey(tag.Element)
}

// (group, element) -> tag information
type Dictionary map[tagDictKey]TagDictEntry

var singletonDict Dictionary

// Create a new, fully filled dictionary.
func init() {
	reader := csv.NewReader(bytes.NewReader([]byte(tagDictData)))
	reader.Comma = '\t'  // tab separated file
	reader.Comment = '#' // comments start with #
	singletonDict = make(Dictionary)
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
		singletonDict[makeTagDictKey(tag)] = TagDictEntry{
			Tag: tag,
			VR:      strings.ToUpper(row[1]),
			Name:    row[2],
			VM:      row[3],
			Version: row[4],
		}
	}
}

// LookupDictionary finds information about tag (group, element). If the given
// tag is undefined or retired in the standard, it returns an error.
func LookupTag(tag Tag) (TagDictEntry, error) {
	entry, ok := singletonDict[makeTagDictKey(tag)]
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

// TagDebugString returns a human-readable diagnostic string for the tag
func TagDebugString(tag Tag) string {
	e, err := LookupTag(tag)
	if err != nil {
		return fmt.Sprintf("(%04x,%04x)[??]", tag.Group, tag.Element)
	}
	return fmt.Sprintf("(%04x,%04x)[??]", tag.Group, tag.Element, e.Name)
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
