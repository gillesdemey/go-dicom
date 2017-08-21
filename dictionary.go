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

type DictionaryEntry struct {
	// group and element are results of parsing the hex-pair tag, such as (1000,10008)
	group   uint16
	element uint16

	// Data encoding
	vr string
	// Human-readable name of the tag
	name    string
	vm      string
	version string
}

// Combination of group and element.
type dictKey uint32

func makeDictKey(group, element uint16) dictKey {
	return (dictKey(group) << 16) | dictKey(element)
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
		group, element, err := splitTag(row[0])
		if err != nil {
			continue // we don't support groups yet
		}

		dict[makeDictKey(group, element)] = DictionaryEntry{
			group:   group,
			element: element,
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
func LookupDictionary(dict Dictionary, group, element uint16) (DictionaryEntry, error) {
	key := makeDictKey(group, element)
	entry, ok := dict[key]

	if !ok {
		// (0000-u-ffff,0000)	UL	GenericGroupLength	1	GENERIC
		if group%2 == 0 && element == 0x0000 {
			entry = DictionaryEntry{group, element, "UL", "GenericGroupLength", "1", "GENERIC"}
		} else {
			return DictionaryEntry{}, ErrTagNotFound
		}
	}
	return entry, nil
}

// Split a tag into a group and element, represented as a hex value
// TODO: support group ranges (6000-60FF,0803)
func splitTag(tag string) (uint16, uint16, error) {
	parts := strings.Split(strings.Trim(tag, "()"), ",")
	group, err := strconv.ParseInt(parts[0], 16, 0)
	if err != nil {
		return 0, 0, err
	}
	elem, err := strconv.ParseInt(parts[1], 16, 0)
	if err != nil {
		return 0, 0, err
	}
	return uint16(group), uint16(elem), nil
}
