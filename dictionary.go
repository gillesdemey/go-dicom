package dicom

import (
	"encoding/csv"
	"github.com/bitbored/go-segtree"
	"io"
	"strconv"
	"strings"
)

type dictEntry struct {
	vr      VR // Value Representation
	name    string
	vm      string // Value Multiplicity
	version string
}

type rangeType int

const (
	single = rangeType(iota)
	even
	odd
	both
)

type rangeNumber struct {
	t     rangeType
	begin uint16
	end   uint16
}

type group struct {
	elements          map[uint16]*dictEntry
	evenElementRanges *segtree.Tree
	oddElementRanges  *segtree.Tree
	bothElementRanges *segtree.Tree
}

type Dictionary struct {
	groups          map[uint16]group
	evenGroupRanges *segtree.Tree
	oddGroupRanges  *segtree.Tree
	bothGroupRanges *segtree.Tree
}

// Parses a CSV Dicom dictionary from r
// Data element tags should be formatted using the conventions in
// DICOM PS3.6 2015c
func ParseDictionary(r io.Reader) (*Dictionary, error) {

	reader := csv.NewReader(r)
	reader.Comma = '\t'  // tab separated file
	reader.Comment = '#' // comments start with #

	d := new(Dictionary)
	d.groups = make(map[uint16]group)

	// Map to hold group ranges to add to the dictionary
	// once the inner segment trees are built
	groupRanges := make(map[rangeNumber]group)

	for {

		row, err := reader.Read()

		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		groupRange, elementRange, err := splitTag(row[0])
		if err != nil {
			return nil, err
		}

		entry := &dictEntry{
			ParseVR(strings.ToUpper(row[1])),
			row[2],
			row[3],
			row[4],
		}

		d.addEntry(groupRanges, groupRange, elementRange, entry)
	}

	// Build segment trees of groups defined by single index
	for _, g := range d.groups {
		g.buildTrees()
	}

	d.addGroupRanges(groupRanges)

	if d.evenGroupRanges != nil {
		if err := d.evenGroupRanges.BuildTree(); err != nil {
			return nil, err
		}
	}
	if d.oddGroupRanges != nil {
		if err := d.oddGroupRanges.BuildTree(); err != nil {
			return nil, err
		}
	}
	if d.oddGroupRanges != nil {
		if err := d.bothGroupRanges.BuildTree(); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (g *group) buildTrees() error {

	if g.evenElementRanges != nil {
		if err := g.evenElementRanges.BuildTree(); err != nil {
			return err
		}
	}
	if g.oddElementRanges != nil {
		if err := g.oddElementRanges.BuildTree(); err != nil {
			return err
		}
	}
	if g.bothElementRanges != nil {
		if err := g.bothElementRanges.BuildTree(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Dictionary) addEntry(groupRanges map[rangeNumber]group, groupRange, elementRange rangeNumber, entry *dictEntry) {

	var g group

	if groupRange.t == single {
		if _, exists := d.groups[groupRange.begin]; exists == false {
			g := group{}
			g.elements = make(map[uint16]*dictEntry)
			d.groups[groupRange.begin] = g
		}
		g = d.groups[groupRange.begin]
	} else {
		if _, exists := groupRanges[groupRange]; exists == false {
			g := group{}
			g.elements = make(map[uint16]*dictEntry)
			groupRanges[groupRange] = g
		}
		g = groupRanges[groupRange]

	}

	switch elementRange.t {
	case single:
		g.elements[elementRange.begin] = entry
	case even:
		if g.evenElementRanges == nil {
			g.evenElementRanges = new(segtree.Tree)
		}
		g.evenElementRanges.Push(int(elementRange.begin), int(elementRange.end), entry)
	case odd:
		if g.oddElementRanges == nil {
			g.oddElementRanges = new(segtree.Tree)
		}
		g.oddElementRanges.Push(int(elementRange.begin), int(elementRange.end), entry)
	case both:
		if g.bothElementRanges == nil {
			g.bothElementRanges = new(segtree.Tree)
		}
		g.bothElementRanges.Push(int(elementRange.begin), int(elementRange.end), entry)
	}
}

func (d *Dictionary) addGroupRanges(groupRanges map[rangeNumber]group) error {
	for groupRange, g := range groupRanges {
		// Build inner segment trees
		if err := g.buildTrees(); err != nil {
			return err
		}

		switch groupRange.t {
		case even:
			if d.evenGroupRanges == nil {
				d.evenGroupRanges = new(segtree.Tree)
			}
			d.evenGroupRanges.Push(int(groupRange.begin), int(groupRange.end), &g)
		case odd:
			if d.oddGroupRanges == nil {
				d.oddGroupRanges = new(segtree.Tree)
			}
			d.oddGroupRanges.Push(int(groupRange.begin), int(groupRange.end), &g)
		case both:
			if d.bothGroupRanges == nil {
				d.bothGroupRanges = new(segtree.Tree)
			}
			d.bothGroupRanges.Push(int(groupRange.begin), int(groupRange.end), &g)
		}
	}

	return nil
}

// Split a tag into a group and element, represented as a hex value
func splitTag(tag string) (groupRange rangeNumber, elementRange rangeNumber, err error) {

	parts := strings.Split(strings.Trim(tag, "()"), ",")

	groupRange, err = getRange(parts[0])
	if err != nil {
		return
	}

	elementRange, err = getRange(parts[1])
	if err != nil {
		return
	}

	return
}

func getRange(r string) (rangeNumber, error) {

	var rng rangeNumber
	rng.t = single

	parts := strings.Split(r, "-")

	if begin, err := strconv.ParseUint(parts[0], 16, 16); err != nil {
		return rng, err
	} else {
		rng.begin = uint16(begin)
	}

	if len(parts) > 1 {
		rng.t = even
		endIndex := 1
		if len(parts) > 2 {
			endIndex = 2
			switch parts[1] {
			case "o":
				rng.t = odd
			case "u":
				rng.t = both
			}
		}

		end, err := strconv.ParseUint(parts[endIndex], 16, 16)
		if err != nil {
			return rng, err
		}
		rng.end = uint16(end)
	} else {
		rng.end = rng.begin
	}

	return rng, nil
}

func (d *Dictionary) getEntry(groupNumber, element uint16) (*dictEntry, error) {

	if g, found := d.groups[groupNumber]; found {
		if e, err := g.getEntry(element); err == nil {
			return e, nil
		} else if err != ErrTagNotFound {
			return nil, err
		}
	}

	if d.bothGroupRanges != nil {
		groups, err := d.bothGroupRanges.QueryIndex(int(groupNumber))
		if err != nil {
			return nil, err
		}

		grp, found := <-groups
		if found {
			g := grp.(*group)
			if e, err := g.getEntry(element); err == nil {
				return e, nil
			} else if err != ErrTagNotFound {
				return nil, err
			}
		}

	}

	if groupNumber%2 == 0 {
		if d.evenGroupRanges != nil {
			groups, err := d.evenGroupRanges.QueryIndex(int(groupNumber))
			if err != nil {
				return nil, err
			}

			grp, found := <-groups
			if found {
				g := grp.(*group)
				if e, err := g.getEntry(element); err == nil {
					return e, nil
				} else if err != ErrTagNotFound {
					return nil, err
				}
			}
		}

	} else {
		if d.oddGroupRanges != nil {
			groups, err := d.oddGroupRanges.QueryIndex(int(groupNumber))
			if err != nil {
				return nil, err
			}

			grp, found := <-groups
			if found {
				g := grp.(*group)
				if e, err := g.getEntry(element); err == nil {
					return e, nil
				} else if err != ErrTagNotFound {
					return nil, err
				}
			}
		}
	}

	return nil, ErrTagNotFound
}

func (g *group) getEntry(element uint16) (*dictEntry, error) {
	if e, found := g.elements[element]; found {
		return e, nil
	}

	if g.bothElementRanges != nil {
		entries, err := g.bothElementRanges.QueryIndex(int(element))
		if err != nil {
			return nil, err
		}
		e, found := <-entries
		if found {
			return e.(*dictEntry), nil
		}
	}

	if element%2 == 0 {
		if g.evenElementRanges != nil {
			entries, err := g.evenElementRanges.QueryIndex(int(element))
			if err != nil {
				return nil, err
			}
			e, found := <-entries
			if found {
				return e.(*dictEntry), nil
			}
		}
	} else {
		if g.oddElementRanges != nil {
			entries, err := g.oddElementRanges.QueryIndex(int(element))
			if err != nil {
				return nil, err
			}
			e, found := <-entries
			if found {
				return e.(*dictEntry), nil
			}
		}
	}

	return nil, ErrTagNotFound
}
