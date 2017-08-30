package dicom

import (
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"log"
)

// Defines how a []byte is translated into a utf8 string.
type CodingSystem struct {
	// VR="PN" is the only place where we potentially use all three
	// decoders.  For all other VR types, only Ideographic decoder is used.
	// See P3.5, 6.2.
	//
	// P3.5 6.1 is supposed to define the coding systems in detail.  But the
	// spec text is insanely obtuse and I couldn't tell what its meaning
	// after hours of trying. So I just copied what pydicom charset.py is
	// doing.
	Alphabetic  *encoding.Decoder
	Ideographic *encoding.Decoder
	Phonetic    *encoding.Decoder
}


type CodingSystemType int

const (
	// See CodingSystem for explanations of these coding-system types.
	AlphabeticCodingSystem = iota
	IdeographicCodingSystem
	PhoneticCodingSystem
)

// Mapping of DICOM charset name to golang encoding/htmlindex name.  "" means
// 7bit ascii.
var htmlEncodingNames = map[string]string{
	"ISO 2022 IR 6":   "",
	"ISO_IR 13":       "shift_jis",
	"ISO 2022 IR 13":  "shift_jis",
	"ISO_IR 100":      "",
	"ISO 2022 IR 100": "",
	"ISO_IR 101":      "iso-8859-2",
	"ISO 2022 IR 101": "iso-8859-2",
	"ISO_IR 109":      "iso-8859-3",
	"ISO 2022 IR 109": "iso-8859-3",
	"ISO_IR 110":      "iso-8859-4",
	"ISO 2022 IR 110": "iso-8859-4",
	"ISO_IR 126":      "iso-ir-126",
	"ISO 2022 IR 126": "iso-ir-126",
	"ISO_IR 127":      "iso-ir-127",
	"ISO 2022 IR 127": "iso-ir-127",
	"ISO_IR 138":      "iso-ir-138",
	"ISO 2022 IR 138": "iso-ir-138",
	"ISO_IR 144":      "iso-ir-144",
	"ISO 2022 IR 144": "iso-ir-144",
	"ISO_IR 148":      "iso-ir-148",
	"ISO 2022 IR 148": "iso-ir-148",
	"ISO 2022 IR 149": "euc-kr",
	"ISO 2022 IR 159": "iso-2022-jp",
	"ISO_IR 166":      "iso-ir-166",
	"ISO 2022 IR 166": "iso-ir-166",
	"ISO 2022 IR 87":  "iso-2022-jp",
}

// Convert DICOM character encoding names, such as "ISO-IR 100" to golang
// decoder. It will return nil, nil for the default (7bit ASCII)
// encoding. Cf. P3.2
// D.6.2. http://dicom.nema.org/medical/dicom/2016d/output/chtml/part02/sect_D.6.2.html
func parseSpecificCharacterSet(elem *DicomElement) (CodingSystem, error) {
	// Set the []byte -> string decoder for the rest of the
	// file.  It's sad that SpecificCharacterSet isn't part
	// of metadata, but is part of regular attrs, so we need
	// to watch out for multiple occurrences of this type of
	// elements.
	encodingNames, err := elem.GetStrings()
	if err != nil {
		return CodingSystem{}, err
	}
	var decoders []*encoding.Decoder
	for _, name := range encodingNames {
		var c *encoding.Decoder
		if htmlName, ok := htmlEncodingNames[name]; !ok {
			// TODO(saito) Support more encodings.
			log.Printf("Unknown character set '%s'. Assuming utf-8", encodingNames[0])
		} else {
			if htmlName != "" {
				d, err := htmlindex.Get(htmlName)
				if err != nil {
					log.Panicf("Encoding name %s (for %s) not found", name, htmlName)
				}
				c = d.NewDecoder()
			}
		}
		decoders = append(decoders, c)
	}
	if len(decoders) == 0 {
		return CodingSystem{nil, nil, nil}, nil
	} else if len(decoders) == 1 {
		return CodingSystem{decoders[0], decoders[0], decoders[0]}, nil
	} else if len(decoders) == 2 {
		return CodingSystem{decoders[0], decoders[1], decoders[1]}, nil
	} else {
		return CodingSystem{decoders[0], decoders[1], decoders[2]}, nil
	}
}
