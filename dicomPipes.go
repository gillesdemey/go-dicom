package dicom

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	fp "path/filepath"
)

const (
	JPEG_2000       = "1.2.840.10008.1.2.4.91"
	JPEG_BASELINE_1 = "1.2.840.10008.1.2.4.50"
)

func fileName(folder string, i int, ext string) string {
	basename := fp.Base(folder)
	filename := basename + "_" + fmt.Sprintf("%03d\n", i)
	return fp.Join(folder, filename) + "." + ext
}

// Writes pixel data to folder
func (di *DicomFile) WriteImagesToFolder(folder string) {
	var inImg bool = false
	var idx int
	var txUID, fext string

	for _, dcmElem := range di.Elements {
		switch dcmElem.Name {
		case "TransferSyntaxUID":
			txUID = dcmElem.Value[0].(string)

		case "PixelData":
			inImg = true

			switch txUID {

			// JPEG baseline 1
			case JPEG_BASELINE_1:
				fext = "jpg"

				// JPEG 2000 Part 1
			case JPEG_2000:
				fext = "jp2"

				// not implemented
			default:
				//panic("Non implemented Transfer Syntax: \"" + txUID + "\"")

			}

		case "Item":
			if inImg == true {

				if idx > 0 {
					pb := dcmElem.Value[0].([]byte)
					err := ioutil.WriteFile(fileName(folder, idx, fext), pb, 0644)
					if err != nil {
						panic(err)
					}
				}

				idx++
			}
		}
	}
}

// Writes dicom elements to file
func (di *DicomFile) WriteToFile(file *os.File) {
	for _, dcmElem := range di.Elements {
		_, err := file.WriteString(fmt.Sprintln(dcmElem))
		if err != nil {
			panic(err)
		}

		file.Close()
	}
}

// Logs dicom elements
func (di *DicomFile) Log() {
	logger := log.New(os.Stdout, "", 0)
	for _, dcmElem := range di.Elements {
		logger.Println(dcmElem)
	}
}
