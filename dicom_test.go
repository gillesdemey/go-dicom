package dicom

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestRead1(t *testing.T) {

	dcmfile, _ := os.Open("examples/IM-0001-0001.dcm")
	dr := NewReader(dcmfile)

	for {
		de, err := dr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		} else {
			if de.Name == "SOPInstanceUID" {
				log.Println("SOPInstanceUID", de)
				log.Println("SOPInstanceUID", de.Values[0].(string))
			}
			if de.Name == "StudyDate" {
				log.Println("StudyDate", de)
				log.Println("StudyDate", de.Values[0])
				log.Println("StudyDate", de.Values[0].(*DcmDA).Date())
			}
			if de.Name == "StudyTime" {
				log.Println("StudyTime", de)
				log.Println("StudyTime", de.Values[0])
				log.Println("StudyTime", de.Values[0].(*DcmTM).Time())
			}
			if de.Name == "WindowCenter" {
				log.Println("WindowCenter", de)
				log.Println("WindowCenter", de.Values[0])
				log.Println("WindowCenter", de.Values[0].(*DcmDS).Float64())
			}
			if de.Name == "GenericGroupLength" {
				log.Println("GenericGroupLength", de)
				log.Println("GenericGroupLength", de.Values[0])
				log.Println("GenericGroupLength", de.Values[0].(*DcmUL).Uint32())
			}
		}
	}
	dcmfile.Close()

}

func TestRead2(t *testing.T) {

	dcmfile, _ := os.Open("examples/IM-0001-0001.dcm")
	dr := NewReader(dcmfile)

	for {
		de, err := dr.Read()

		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Fatal(err)
			}
		} else {
			log.Println(de)
		}
	}
	dcmfile.Close()

}

func TestReadAllFiles(t *testing.T) {

	inputFolder := "examples/"

	files, err := ioutil.ReadDir(inputFolder)
	if err != nil {
		panic(err)
	}

	for _, file := range files {

		log.Println("filename " + file.Name())
		dcmfile, _ := os.Open(inputFolder + file.Name())
		dr := NewReader(dcmfile)
		for {
			de, err := dr.Read()

			if err != nil {
				if err == io.EOF {
					break
				} else {
					log.Fatal(err)
				}
			} else {
				log.Println(de)
			}
		}
		dcmfile.Close()

	}

}

func BenchmarkParseSingle(b *testing.B) {

	dcmfile, _ := os.Open("examples/IM-0001-0001.dcm")
	dr := NewReader(dcmfile)

	for {
		de, err := dr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		} else {
			log.Println(de)
		}
	}
	dcmfile.Close()
}
