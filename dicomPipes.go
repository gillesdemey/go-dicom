package dicom

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	fp "path/filepath"
	"strconv"
	"sync"
)

type DicomMessage struct {
	msg  *DicomElement
	wait chan bool
}

const (
	JPEG_2000       = "1.2.840.10008.1.2.4.91"
	JPEG_BASELINE_1 = "1.2.840.10008.1.2.4.50"
)

// DicomMessage channel Generator
// DicomMessage contains dicom elements parsed from dicom file.
func (di *DicomFile) Parse(buff []byte) <-chan DicomMessage {

	parser, err := NewParser()
	if err != nil {
		panic(err)
	}
	di.parser = parser

	_, c := parser.Parse(buff)
	if err != nil {
		panic(err)
	}
	return c
}

// Discard messages
func (di *DicomFile) Discard(in <-chan DicomMessage, done *sync.WaitGroup) {
	done.Add(1)
	go func() {
		for dcmMsg := range in {
			dcmMsg.wait <- true
		}
		done.Done()
	}()

}

func filePathNameMultiple(folder, fn string, i int, ext string) string {
	filename := fn + "_" + fmt.Sprintf("%03d", i) + "." + ext
	return fp.Join(folder, filename)
}

func filePathNameUnique(folder, fn string, ext string) string {
	filename := fn + "." + ext
	return fp.Join(folder, filename)
}

// Consumer that writes pixel data to files
// SOPInstanceUID identifies image with uniqueness
func (di *DicomFile) WriteImagesToFile(in <-chan DicomMessage, done *sync.WaitGroup, folder string) <-chan DicomMessage {

	out := make(chan DicomMessage)
	waitMsg := make(chan bool)

	done.Add(1)
	go func() {

		var inImg bool = false
		var idx int
		var numberOfFrames int = 1
		var txUID, instanceUID, filename, fext string

		for dcmMsg := range in {

			switch dcmMsg.msg.Name {

			case "TransferSyntaxUID":
				txUID = dcmMsg.msg.Value[0].(string)

			case "NumberOfFrames":
				numberOfFrames, _ = strconv.Atoi(dcmMsg.msg.Value[0].(string))

			case "SOPInstanceUID":
				instanceUID = dcmMsg.msg.Value[0].(string)

			case "PixelData":
				inImg = true

				switch txUID {

				//JPEG baseline 1
				case JPEG_BASELINE_1:
					fext = "jpg"

				//JPEG 2000 Part 1
				case JPEG_2000:
					fext = "jp2"

				// not implemented
				default:
					//panic("Non imlpemented Transfer Syntax: \"" + txUID + "\"")
				}

			case "Item":
				if inImg == true {

					if idx > 0 {
						pb := dcmMsg.msg.Value[0].([]byte)
						if numberOfFrames == 1 {
							filename = filePathNameUnique(folder, instanceUID, fext)
						} else {
							filename = filePathNameMultiple(folder, instanceUID, idx, fext)
						}
						err := ioutil.WriteFile(filename, pb, 0644)
						if err != nil {
							panic(err)
						}
					}

					idx++
				}
			}

			out <- DicomMessage{dcmMsg.msg, waitMsg}
			<-waitMsg
			dcmMsg.wait <- true
		}
		close(out)
		done.Done()
	}()
	return out

}

func removeFile(fn string) {
	if _, err := os.Stat(fn); os.IsExist(err) {
		err := os.Remove(fn)
		if err != nil {
			panic(err)
		}
	}
}

// Consumer that writes dicom elements to file
// SOPInstanceUID identifies image with uniqueness
func (di *DicomFile) WriteLogToFile(in <-chan DicomMessage, done *sync.WaitGroup, folder string) <-chan DicomMessage {

	out := make(chan DicomMessage)
	waitMsg := make(chan bool)
	done.Add(1)

	go func() {
		//var f1 *os.File
		var instanceUID string

		// Initially writes the log to a temporary file because cannot determine
		// the filename until SOPInstanceUID is not received from the "in" channel

		// create a randomic name of file to avoid the tentative of use the same
		// filename due to concurrency
		fn1 := folder + strconv.Itoa(rand.Int()) + "_tmp.txt"

		// Ensures fn1 does not exist in folder
		removeFile(fn1)

		f1, err := os.Create(fn1)
		if err != nil {
			panic(err)
		}

		for dcmMsg := range in {
			if dcmMsg.msg.Name == "SOPInstanceUID" {
				instanceUID = dcmMsg.msg.Value[0].(string)
			}

			_, err := f1.WriteString(fmt.Sprintln(dcmMsg.msg))
			if err != nil {
				panic(err)
			}

			out <- DicomMessage{dcmMsg.msg, waitMsg}
			<-waitMsg
			dcmMsg.wait <- true

		}
		f1.Close()

		// rename "_tmp.txt" to the definitive filename
		fn2 := folder + instanceUID + ".txt"
		os.Rename(fn1, fn2)
		removeFile(fn1)

		close(out)
		done.Done()
	}()
	return out

}

// Consumer that logs dicom elements
func (di *DicomFile) Log(in <-chan DicomMessage, done *sync.WaitGroup) <-chan DicomMessage {
	logger := log.New(os.Stdout, "", 0)
	out := make(chan DicomMessage)
	waitMsg := make(chan bool)

	done.Add(1)
	go func() {
		for dcmMsg := range in {
			logger.Println(dcmMsg.msg)
			out <- DicomMessage{dcmMsg.msg, waitMsg}
			<-waitMsg
			dcmMsg.wait <- true
		}
		close(out)
		done.Done()
	}()
	return out
}
