package main

import (
	"flag"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strings"
	"sync"
)

var (
	file   = flag.String("file", "", "the DICOM file you want to parse")
	silent = flag.Bool("silent", false, "wether or not to print all Data Elements")
	out    = flag.String("out", "", "where to write the program's output")
	folder = flag.String("folder", "", "Folder with DICOM images to extract")
)

func init() {
	flag.Parse()
}

func main() {
	// file input
	if *file != "" {
		processFile(*file, silent, out)
	}

	// folder input, find .dcm files
	if *folder != "" {
		err := fp.Walk(*folder, fileWalker)
		if err != nil {
			panic(err)
		}
	}
}

func fileWalker(path string, info os.FileInfo, err error) error {
	if err != nil {
		panic(err)
	}

	// don't parse nested directories
	if info.IsDir() {
		return nil
	}

	// not a DICOM file
	if fp.Ext(info.Name()) != ".dcm" {
		return nil
	}

	processFile(path, silent, out)

	return err
}

func processFile(path string, silent *bool, out *string) {
	buff, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	done := new(sync.WaitGroup)
	done.Add(1)

	parser, err := dicom.NewParser()
	if err != nil {
		panic(err)
	}

	dcm, err := parser.Parse(buff)
	if err != nil {
		panic(err)
	}

	if *silent == false {
		dcm.Log()
	}

	if *out != "" {
		basename := fp.Base(path)
		filename := strings.TrimSuffix(basename, fp.Ext(basename))
		outDir := fp.Join(*out, filename)

		// ensure out directory exists
		err := os.MkdirAll(outDir, 0755)
		if err != nil {
			panic(err)
		}

		elemsFile, err := os.Create(fp.Join(outDir, filename+".txt"))
		if err != nil {
			panic(err)
		}
		dcm.WriteToFile(elemsFile)
		dcm.WriteImagesToFolder(outDir)
	}
}
