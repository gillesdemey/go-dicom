package dicom

// Dictionary supports looking up DICOM data dictionary as defined in
//
// ftp://medical.nema.org/medical/dicom/2011/11_06pu.pdf

import (
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

type DictionaryEntry struct {
	tag Tag

	// Data encoding
	vr string
	// Human-readable name of the tag
	name    string
	vm      string
	version string
}


// Combination of group and element.
type dictKey uint32

func makeDictKey(tag Tag) dictKey {
	return (dictKey(tag.Group) << 16) | dictKey(tag.Element)
}

// (group, element) -> tag information
type Dictionary map[dictKey]DictionaryEntry

// Create a new, fully filled dictionary.
func NewDictionary() Dictionary {
	reader := csv.NewReader(bytes.NewReader([]byte(dicomDictData)))
	reader.Comma = '\t'  // tab separated file
	reader.Comment = '#' // comments start with #
	dict := make(Dictionary)
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

		dict[makeDictKey(tag)] = DictionaryEntry{
			tag: tag,
			vr:      strings.ToUpper(row[1]),
			name:    row[2],
			vm:      row[3],
			version: row[4],
		}
	}
	return dict
}

// LookupDictionary finds information about tag (group, element). If the given
// tag is undefined or retired in the standard, it returns an error.
func LookupDictionary(dict Dictionary, tag Tag) (DictionaryEntry, error) {
	entry, ok := dict[makeDictKey(tag)]

	if !ok {
		// (0000-u-ffff,0000)	UL	GenericGroupLength	1	GENERIC
		if tag.Group%2 == 0 && tag.Element == 0x0000 {
			entry = DictionaryEntry{tag, "UL", "GenericGroupLength", "1", "GENERIC"}
		} else {
			return DictionaryEntry{}, ErrTagNotFound
		}
	}
	return entry, nil
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
