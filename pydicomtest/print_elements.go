package main

// Print the contents of a file in a format used by pydicom.  Used to ensure
// that pydicom and go-dicom parses files in the same way.

import (
	"flag"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"io/ioutil"
	"log"
	"sort"
	"strings"
)

var (
	printMetadata = flag.Bool("print-metadata", true, "Print image metadata")
)

// Sorter
type elemSorter struct {
	elems []dicom.DicomElement
}

func (e *elemSorter) Len() int {
	return len(e.elems)
}

func (e *elemSorter) Swap(i, j int) {
	tmp := e.elems[i]
	e.elems[i] = e.elems[j]
	e.elems[j] = tmp
}

func (e *elemSorter) Less(i, j int) bool {
	elemi := e.elems[i]
	elemj := e.elems[j]
	if elemi.Tag.Group < elemj.Tag.Group {
		return true
	}
	if elemi.Tag.Group > elemj.Tag.Group {
		return false
	}
	return elemi.Tag.Element < elemj.Tag.Element
}

func main() {
	flag.Parse()
	if len(flag.Args()) == 0 {
		log.Panic("print_elements_test <dicomfile>")
	}
	path := flag.Arg(0)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	data, err := dicom.ParseBytes(bytes)
	if err != nil {
		panic(err)
	}

	printElements(data.Elements, 0)
}

func printScalar(i interface{}, indent int) string {
	var s string
	switch v := i.(type) {
	case float32:
		s = fmt.Sprintf("%.1f", v)
	case float64:
		s = fmt.Sprintf("%.1f", v)
	case string:
		if indent == 0 {
			s = fmt.Sprintf("'%s'", v)
		} else {
			s = fmt.Sprintf("%v", i)
		}
	default:
		s = fmt.Sprintf("%v", i)
	}
	return s
}

func printTag(tag dicom.Tag) string {
	return fmt.Sprintf("(%04x, %04x)", tag.Group, tag.Element)
}

func printElement(elem *dicom.DicomElement, indent int) {
	if elem.Tag.Group == 2 {
		// Don't print the meta elements found in the beginning of the
		// file. Pydicom doesn't for some reason.
		return
	}

	fmt.Printf("%s%s %s:", strings.Repeat(" ", indent*2), printTag(elem.Tag), elem.Vr)
	if elem.Vr == "OW" || elem.Vr == "OB" || elem.Vr == "OD" || elem.Vr == "OF" || elem.Vr == "LO" {
		if len(elem.Value) != 1 {
			fmt.Printf(" [%d values]", len(elem.Value))
		} else if v, ok := elem.Value[0].([]byte); ok {
			fmt.Printf(" %dbytes\n", len(v))
		} else {
			v := elem.Value[0].(string)
			fmt.Printf(" %dbytes\n", len(v))
		}
	} else if elem.Vr == "LT" {
		// pydicom trims one (but not more) trailing space from the
		// string.  Whereas go-dicom doesn't trim anything. The spec
		// says an impl "*may* trim trailing space*s* from the string".
		// So the behavior of pydicom is a bit strange, but follows the
		// wording.
		v := elem.Value[0].(string)
		n := len(v)
		if strings.HasSuffix(v, " ") {
			n--
		}
		fmt.Printf(" %dbytes\n", n)
	} else if elem.Vr == "UI" {
		// Resolve UIDs if possible.
		uid := dicom.MustGetString(*elem)
		e, err := dicom.LookupUID(uid)
		if err == nil {
			uid = e.Name
		}
		fmt.Printf(" %s\n", uid)
	} else if elem.Vr == "AT" {
		if len(elem.Value) != 1 {
			log.Panic(elem)
		}
		tag := elem.Value[0].(dicom.Tag)
		fmt.Printf(" %s\n", printTag(tag))
	} else if elem.Vr != "SQ" { // not a sequence
		if len(elem.Value) == 0 {
			fmt.Print("\n")
		} else if len(elem.Value) == 1 {
			fmt.Printf(" %s\n", printScalar(elem.Value[0], 1))
		} else {
			fmt.Print(" [")
			for i, value := range elem.Value {
				if i > 0 {
					fmt.Print(", ")
				}
				// Follow the pydicom's printing format.  It
				// encloses the value in '...' only at the
				// toplevel.
				if indent == 0 {
					//fmt.Print("'")
				}
				fmt.Print(printScalar(value, indent))
				if indent == 0 {
					//fmt.Print("'")
				}
			}
			fmt.Print("]\n")
		}
	} else {
		var childElems []dicom.DicomElement
		if len(elem.Value) == 1 {
			// If SQ contains one Item, unwrap the item.
			items := elem.Value[0].(*dicom.DicomElement)
			if items.Tag != dicom.TagItem {
				log.Panicf("A SQ item must be of type Item, but found %v", items)
			}
			for _, item := range items.Value {
				childElems = append(childElems, *item.(*dicom.DicomElement))
			}
		} else {
			for _, v := range elem.Value {
				child := v.(*dicom.DicomElement)
				if child.Tag != dicom.TagItem {
					log.Panicf("A SQ item must be of type Item, but found %v", child)
				}
				childElems = append(childElems, *child)
			}
		}
		fmt.Print("\n")
		printElements(childElems, indent+1)
	}
}

func printElements(elems []dicom.DicomElement, indent int) {
	sort.Sort(&elemSorter{elems: elems})
	for _, elem := range elems {
		printElement(&elem, indent)
	}
}
