package main

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

func printElements(elems []dicom.DicomElement, indent int) {
	sort.Sort(&elemSorter{elems: elems})
	for _, elem := range elems {
		fmt.Printf("%s%s %s", strings.Repeat(" ", indent), elem.Tag, elem.Vr)
		if elem.Vr == "OW" || elem.Vr == "OB" || elem.Vr == "OD" || elem.Vr == "OF" || elem.Vr == "LT" || elem.Vr == "LO" {
			fmt.Printf("%dbytes\n", len(elem.Value))
		} else if elem.Vr != "SQ" { // not a sequence
			if len(elem.Value) == 1 {
				fmt.Print(elem.Value[0])
			} else {
				fmt.Print("[")
				for i, value := range elem.Value {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Print(value)
				}
			}
			fmt.Print("\n")
		} else {
			var childElems []dicom.DicomElement
			for _, v := range elem.Value {
				child := v.(*dicom.DicomElement)
				childElems = append(childElems, *child)
			}
			fmt.Print("\n")
			printElements(childElems, indent+1)
		}
	}
}
