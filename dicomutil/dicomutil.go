package main

import (
	"flag"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"io/ioutil"
	"log"
	"os"
)

var (
	printMetadata = flag.Bool("print-metadata", true, "Print image metadata")
	extractImages = flag.Bool("extract-images", false, "Extract images into separate files")
)

func main() {
	flag.Parse()
	if len(flag.Args()) == 0 {
		log.Panic("dicomutil <dicomfile>")
	}
	path := flag.Arg(0)
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	st, err := file.Stat()
	if err != nil {
		panic(err)
	}
	parser := dicom.NewParser()
	data, err := parser.Parse(file, st.Size())
	if err != nil {
		panic(err)
	}
	if *printMetadata {
		for i, elem := range data.Elements {
			fmt.Printf("Element %d: %v\n", i, elem.String())
		}
	}
	if *extractImages {
		n := 0
		for _, elem := range data.Elements {
			if elem.Tag == dicom.TagPixelData.Tag {
				data := elem.Value[0].([]byte)
				path := fmt.Sprintf("image.%d.jpg", n) // TODO: figure out the image format
				n++
				ioutil.WriteFile(path, data, 0644)
				fmt.Printf("%s: %d bytes\n", path, len(data))
			}
		}
	}
}
