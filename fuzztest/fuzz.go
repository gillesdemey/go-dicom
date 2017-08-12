package fuzz

import (
	"github.com/yasushi-saito/go-dicom"
)

func Fuzz(data []byte) int {
	parser, err := dicom.NewParser()
	if err != nil {
		panic(err)
	}
	_, err = parser.Parse(data)
	return 1
}
