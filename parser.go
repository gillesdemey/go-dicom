package dicom

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// Constants
const (
	pixeldata_group    = 0xFFFE
	unknown_group_name = "Unknown Group"
	private_group_name = "Private Data"
)

// Value Multiplicity PS 3.5 6.4
type dcmVM struct {
	s   string
	Min uint8
	Max uint8
	N   bool
}

// A DICOM element
type DicomElement struct {
	Group       uint16
	Element     uint16
	Name        string
	Vr          string
	Vl          uint32
	Value       []interface{} // Value Multiplicity PS 3.5 6.4
	IndentLevel uint8
	elemLen     uint32
	undefLen    bool
	P           uint32
}

type Parser struct {
	dictionary          [][]*dictEntry
	dictionaryNameIndex map[string]*dictEntry
}

// Stringer
func (e *DicomElement) String() string {
	s := strings.Repeat(" ", int(e.IndentLevel)*2)
	sv := fmt.Sprintf("%v", e.Value)
	if len(sv) > 50 {
		sv = sv[1:50] + "(...)"
	}
	sVl := fmt.Sprintf("%d", e.Vl)
	if e.undefLen == true {
		sVl = "UNDEF"
	}

	return fmt.Sprintf("%08d %s (%04X, %04X) %s %s %d %s %s", e.P, s, e.Group, e.Element, e.Vr, sVl, e.elemLen, e.Name, sv)
}

// Return the tag as a string to use in the Dicom dictionary
func (e *DicomElement) getTag() string {
	return fmt.Sprintf("(%04X,%04X)", e.Group, e.Element)
}

// Create a new parser, with functional options for configuration
// http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
func NewParser(options ...func(*Parser) error) (*Parser, error) {

	p := Parser{}

	// apply defaults
	dict := bytes.NewReader([]byte(dicomDictData))
	err := Dictionary(dict)(&p)

	if err != nil {
		panic(err)
	}

	// override defaults
	for _, option := range options {
		err := option(&p)
		if err != nil {
			panic(err)
		}
	}

	return &p, nil
}

func nomalizeDT(str string) string {

	tz := func(str string) (string, string) {
		p := strings.IndexAny(str, "+-")
		if p > -1 {
			return str[:p], (str[p:] + "00")[0:5]
		} else {
			return str, "+0000"
		}
	}

	var strZ string
	var strFrac string
	var strDT string

	fmt.Println("-", str)
	strs := strings.Split(str, ".")
	strDT = strs[0]
	if len(strs) > 1 {
		// has fraction
		strFrac, strZ = tz(strs[1])
	} else {
		strDT, strZ = tz(strs[0])
	}
	p := strings.IndexAny(strDT, "+-")
	if p > -1 {
		strDT = strDT[:p]
	}

	strFrac = "." + (strFrac + "000000")[0:6]
	strR := strDT + strFrac + strZ
	strR = strR[0:4] + "/" + strR[4:6] + "/" + strR[6:8] + " " +
		strR[8:10] + ":" + strR[10:12] + ":" + strR[12:14] + "." +
		strFrac + " " + strR[14:19]

	return strR
}

// Read a DICOM data element
func (buffer *dicomBuffer) readDataElement(p *Parser) *DicomElement {

	implicit := buffer.implicit
	inip := buffer.p
	elem := buffer.readTag(p)

	var vr string     // Value Representation
	var vl uint32 = 0 // Value Length

	// The elements for group 0xFFFE should be Encoded as Implicit VR.
	// DICOM Standard 09. PS 3.6 - Section 7.5: "Nesting of Data Sets"
	if elem.Group == pixeldata_group {
		implicit = true
	}

	if implicit {
		vr, vl = buffer.readImplicit(elem, p)
	} else {
		vr, vl = buffer.readExplicit(elem)
	}

	elem.Vr = vr
	elem.Vl = vl

	// data
	var data []interface{}
	uvl := vl
	valLen := uint32(vl)

	for uvl > 0 {
		switch vr {
		case "AT":
			valLen = 2
			data = append(data, buffer.readHex())

		//TODO:  DA, DT, TM
		// implement Range Matching and Specific Character Set (0008,0005) see PS3.4 C.2.2.2
		case "DA":
			valLen = vl
			str := strings.TrimRight(buffer.readString(vl), " ")
			dcmVal, _ := time.Parse("2006/01/02", str[0:4]+"/"+str[4:6]+"/"+str[6:8])
			data = append(data, dcmVal)

		case "TM":
			//HHMMSS.FFFFFF
			valLen = vl
			str := strings.TrimRight(buffer.readString(vl), " ")
			strs := strings.Split(str, ".")
			if len(strs) == 1 {
				strs = append(strs, "")
			}
			dcmValT, _ := time.Parse("15:04:05.999999", strs[0][0:2]+":"+strs[0][2:4]+":"+strs[0][4:6]+"."+(strs[1] + "000000")[0:6])
			data = append(data, dcmValT)

		case "DT":
			//  YYYYMMDDHHMMSS.FFFFFF&ZZXX where .FFFFFF and &ZZXX are optional
			valLen = vl
			str := strings.TrimRight(buffer.readString(vl), " ")
			dcmValT, _ := time.Parse("2006/01/02 15:04:05.999999 -0700", nomalizeDT(str))
			data = append(data, dcmValT)

		case "UL":
			valLen = 4
			data = append(data, buffer.readUInt32())
		case "SL":
			valLen = 4
			data = append(data, buffer.readInt32())
		case "US":
			valLen = 2
			data = append(data, buffer.readUInt16())
		case "SS":
			valLen = 2
			data = append(data, buffer.readInt16())
		case "FL":
			valLen = 4
			data = append(data, buffer.readFloat())
		case "FD":
			valLen = 8
			data = append(data, buffer.readFloat64())
		case "OW":
			valLen = vl
			data = append(data, buffer.readUInt16Array(vl))
		case "OB":
			valLen = vl
			data = append(data, buffer.readUInt8Array(vl))
		case "NA":
			valLen = vl
		//case "XS": ??

		case "SQ":
			valLen = vl
			data = append(data, "")
		default:
			valLen = vl
			str := strings.TrimRight(buffer.readString(vl), " ")
			strs := strings.Split(str, "\\")
			for _, s := range strs {
				data = append(data, s)
			}

		}
		uvl -= valLen
	}

	elem.P = inip
	elem.Value = data
	elem.elemLen = buffer.p - inip

	return elem
}
