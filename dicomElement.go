package dicom

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A DICOM element
type DicomElement struct {
	group       uint16
	element     uint16
	Name        string
	vr          string
	vl          uint32
	Values      elementValues // Value Multiplicity PS 3.5 6.4
	indentLevel uint8
	elemLen     uint32
	undefLen    bool
	p           uint32
}

type elementValues []elementValue

type elementValue interface{}

// Stringer
func (e *DicomElement) String() string {
	s := strings.Repeat(" ", int(e.indentLevel)*2)
	sv := fmt.Sprintf("%v", e.Values)
	if len(sv) > 80 {
		sv = sv[1:80] + "(...)"
	}
	sVl := fmt.Sprintf("%d", e.vl)
	if e.undefLen == true {
		sVl = "UNDEF"
	}

	return fmt.Sprintf("%08d %s (%04X, %04X) %s %s %d %s %s", e.p, s, e.group, e.element, e.vr, sVl, e.elemLen, e.Name, sv)
}

// Return the tag as a string to use in the Dicom dictionary
func (e *DicomElement) getTag() string {
	return fmt.Sprintf("(%04X,%04X)", e.group, e.element)
}

// ====================================================

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

// ---------------------------------------------------------------------------------------------

type DcmDA struct {
	str  string //YYYYMMDD
	date time.Time
}

func (e *DcmDA) readData(buf []byte) {
	e.str = string(buf)
	e.date, _ = time.Parse("2006/01/02", e.str[0:4]+"/"+e.str[4:6]+"/"+e.str[6:8])
}

func (e *DcmDA) Date() time.Time {
	return e.date
}

func (e *DcmDA) String() string {
	return e.date.Format("Mon, 02 Jan 2006")
}

// ---------------------------------------------------------------------------------------------

type DcmTM struct {
	str  string //HHMMSS.FFFFFF
	date time.Time
}

func (e *DcmTM) readData(buf []byte) {
	e.str = strings.TrimRight(string(buf), " ")
	strs := strings.Split(e.str, ".")
	if len(strs) == 1 {
		strs = append(strs, "")
	}
	e.date, _ = time.Parse("15:04:05.999999", strs[0][0:2]+":"+strs[0][2:4]+":"+strs[0][4:6]+"."+(strs[1] + "000000")[0:6])
}
func (e *DcmTM) Time() time.Time {
	return e.date
}

func (e *DcmTM) String() string {
	return e.date.Format("15:04:05 MST")
}

// ---------------------------------------------------------------------------------------------

type DcmDT struct {
	str  string //  YYYYMMDDHHMMSS.FFFFFF&ZZXX where .FFFFFF and &ZZXX are optional
	date time.Time
}

func (e *DcmDT) readData(buf []byte) {
	e.str = strings.TrimRight(string(buf), " ")
	e.date, _ = time.Parse("2006/01/02 15:04:05.999999 -0700", nomalizeDT(e.str))
}
func (e *DcmDT) DateTime() time.Time {
	return e.date
}

func (e *DcmDT) String() string {
	return e.date.Format(time.RFC1123)
}

// ---------------------------------------------------------------------------------------------

type DcmDS struct {
	str string //  ANSI X3.9,
	val float64
}

func (e *DcmDS) readData(buf []byte) {
	e.str = strings.TrimRight(string(buf), " ")
	e.val, _ = strconv.ParseFloat(e.str, 64)
}

func (e *DcmDS) Float64() float64 {
	return e.val
}

func (e *DcmDS) String() string {
	return fmt.Sprintf("%f", e.val)
}

// ---------------------------------------------------------------------------------------------

// Unsigned Long
//   Unsigned binary integer 32 bits long. Represents an integer n inthe range:
//   0 <= n < 2^32.
type DcmUL struct {
	val uint32
}

func (e *DcmUL) readData(val uint32) {
	e.val = val
}

func (e *DcmUL) Uint32() uint32 {
	return e.val
}

func (e *DcmUL) String() string {
	return fmt.Sprintf("%d", e.val)
}
