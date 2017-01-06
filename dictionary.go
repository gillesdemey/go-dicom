package dicom

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type dictEntry struct {
	g       uint16
	e       uint16
	tag     string
	vr      string
	name    string
	vm      string
	version string
}

// Value Multiplicity PS 3.5 6.4
//   Maybe in the future
/*type dcmVM struct {
	s   string
	Min uint8
	Max uint8
	N   bool
}*/

// Create a new parser, with functional options for configuration
// http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
func NewParser(options ...func(*Parser) error) (*Parser, error) {

	p := Parser{}

	// apply defaults
	dict := bytes.NewReader([]byte(dicomDictData))
	err := Dictionary(dict)(&p)

	if err != nil {
		panic(err)
	}

	// override defaults
	for _, option := range options {
		err := option(&p)
		if err != nil {
			panic(err)
		}
	}

	return &p, nil
}

type Parser struct {
	dictionary          [][]*dictEntry
	dictionaryNameIndex map[string]*dictEntry
}

// Sets the dictionary for the Parser
func Dictionary(r io.Reader) func(*Parser) error {

	return func(p *Parser) error {

		reader := csv.NewReader(r)
		reader.Comma = '\t'  // tab separated file
		reader.Comment = '#' // comments start with #

		dictionary := make([][]*dictEntry, 0xffff+1)
		dictionaryNameIndex := make(map[string]*dictEntry, 0xffff+1)

		for {

			row, err := reader.Read()

			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}

			group, element, err := splitTag(row[0])

			if err != nil {
				// return err
				continue // we don't support groups yet
			}

			if cap(dictionary[group]) == 0 {
				dictionary[group] = make([]*dictEntry, 0xffff+1)
			}

			dictionary[group][element] = &dictEntry{
				uint16(group),
				uint16(element),
				row[0],
				strings.ToUpper(row[1]),
				row[2],
				row[3],
				row[4],
			}

			dictionaryNameIndex[row[2]] = dictionary[group][element]
		}

		p.dictionary = dictionary
		p.dictionaryNameIndex = dictionaryNameIndex
		return nil
	}

}

func (p *Parser) getDictEntry(group, element uint16) (*dictEntry, error) {

	var entry *dictEntry

	tag := fmt.Sprintf("(%s,%s)", group, element)

	// does the entry exist?
	exists := p.dictionary[group] != nil && p.dictionary[group][element] != nil

	if exists {
		entry = p.dictionary[group][element]
	}

	if !exists {
		// (0000-u-ffff,0000)	UL	GenericGroupLength	1	GENERIC
		if group%2 == 0 && element == 0x0000 {
			entry = &dictEntry{group, 0, tag, "UL", "GenericGroupLength", "1", "GENERIC"}
		}
	}

	// nope, still nothing
	if entry == nil {
		return nil, ErrTagNotFound
	}

	return entry, nil
}

// Split a tag into a group and element, represented as a hex value
// TODO: support group ranges (6000-60FF,0803)
func splitTag(tag string) (int64, int64, error) {

	parts := strings.Split(strings.Trim(tag, "()"), ",")

	group, err := strconv.ParseInt(parts[0], 16, 0)
	if err != nil {
		return 0, 0, err
	}
	elem, err := strconv.ParseInt(parts[1], 16, 0)
	if err != nil {
		return 0, 0, err
	}

	return group, elem, nil
}
