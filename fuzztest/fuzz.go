package fuzz

import (
	"bytes"
	"github.com/yasushi-saito/go-dicom"
)

func Fuzz(data []byte) int {
	_, _ = dicom.Parse(bytes.NewBuffer(data), int64(len(data)))
	return 1
}
