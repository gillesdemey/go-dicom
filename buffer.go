package dicom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/text/encoding"
	"io"
)

type transferSyntaxStackEntry struct {
	bo       binary.ByteOrder
	implicit IsImplicitVR
}

type Encoder struct {
	err error

	// TODO(saito) Change to take the io.Writer instead of bytes.Buffer.
	buf *bytes.Buffer
	bo  binary.ByteOrder

	// "implicit" isn't used by Encoder internally. It's there for the user
	// of Encoder to see the current transfer syntax.
	implicit IsImplicitVR

	// Stack of old transfer syntaxes. Used by {Push,Pop}TransferSyntax.
	oldTransferSyntaxes []transferSyntaxStackEntry
}

func NewEncoder(bo binary.ByteOrder, implicit IsImplicitVR) *Encoder {
	return &Encoder{
		err:      nil,
		buf:      &bytes.Buffer{},
		bo:       bo,
		implicit: implicit,
	}
}

// Get the current transfer syntax.
func (e *Encoder) TransferSyntax() (binary.ByteOrder, IsImplicitVR) {
	return e.bo, e.implicit
}

// Temporarily change the encoding format. PopTrasnferSyntax() will restore the
// old format.
func (d *Encoder) PushTransferSyntax(bo binary.ByteOrder, implicit IsImplicitVR) {
	d.oldTransferSyntaxes = append(d.oldTransferSyntaxes,
		transferSyntaxStackEntry{d.bo, d.implicit})
	d.bo = bo
	d.implicit = implicit
}

// Restore the encoding format active before the last call to
// PushTransferSyntax().
func (d *Encoder) PopTransferSyntax() {
	e := d.oldTransferSyntaxes[len(d.oldTransferSyntaxes)-1]
	d.bo = e.bo
	d.implicit = e.implicit
	d.oldTransferSyntaxes = d.oldTransferSyntaxes[:len(d.oldTransferSyntaxes)-1]
}

// Set the error to be reported by future Error() or Finish() calls.
//
// REQUIRES: err != nil
func (e *Encoder) SetError(err error) {
	if e.err == nil {
		e.err = err
	}
}

// Finish() must be called after all the data are encoded.  It returns the
// serialized payload, or error if any.
func (e *Encoder) Finish() ([]byte, error) {
	doassert(len(e.oldTransferSyntaxes) == 0)
	return e.buf.Bytes(), e.err
}

func (e *Encoder) EncodeByte(v byte) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeUInt16(v uint16) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeUInt32(v uint32) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeInt16(v int16) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeInt32(v int32) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeFloat32(v float32) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeFloat64(v float64) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeString(v string) {
	e.buf.Write([]byte(v))
}

// Encode an array of zero bytes.
func (e *Encoder) EncodeZeros(len int) {
	// TODO(saito) reuse the buffer!
	zeros := make([]byte, len)
	e.buf.Write(zeros)
}

// Copy the given data to the output.
func (e *Encoder) EncodeBytes(v []byte) {
	e.buf.Write(v)
}

type IsImplicitVR int

const (
	ImplicitVR IsImplicitVR = iota
	ExplicitVR

	// UnknownVR is to be used when you never encode or decode DataElement.
	UnknownVR
)

type CodingSystemType int

const (
	AlphabeticCodingSystem = iota
	IdeographicCodingSystem
	PhoneticCodingSystem
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

type Decoder struct {
	in  io.Reader
	err error
	bo  binary.ByteOrder
	// "implicit" isn't used by Decoder internally. It's there for the user
	// of Decoder to see the current transfer syntax.
	implicit IsImplicitVR
	// Max bytes to read from "in".
	limit int64
	// Cumulative # bytes read.
	pos int64

	// For decoding raw strings in DICOM file into utf-8.
	// If nil, assume ASCII. Cf P3.5 6.1.2.1
	codingSystem CodingSystem

	// Stack of old transfer syntaxes. Used by {Push,Pop}TransferSyntax.
	oldTransferSyntaxes []transferSyntaxStackEntry
	// Stack of old limits. Used by {Push,Pop}Limit.
	// INVARIANT: oldLimits[] store values in decreasing order.
	oldLimits []int64
}

// NewDecoder creates a decoder object that reads up to "limit" bytes from "in".
// Don't pass just an arbitrary large number as the "limit". The underlying code
// assumes that "limit" accurately bounds the end of the data.
func NewDecoder(
	in io.Reader,
	limit int64,
	bo binary.ByteOrder,
	implicit IsImplicitVR) *Decoder {
	return &Decoder{
		in:       in,
		err:      nil,
		bo:       bo,
		implicit: implicit,
		pos:      0,
		limit:    limit,
	}
}

// Create a decoder that reads from a sequence of bytes. See NewDecoder() for
// explanation of other parameters.
func NewBytesDecoder(data []byte, bo binary.ByteOrder, implicit IsImplicitVR) *Decoder {
	return NewDecoder(bytes.NewBuffer(data), int64(len(data)), bo, implicit)
}

// Set the error to be reported by future Error() or Finish() calls.
//
// REQUIRES: err != nil
func (d *Decoder) SetError(err error) {
	if d.err == nil {
		d.err = err
	}
}

// Get the current transfer syntax.
func (d *Decoder) TransferSyntax() (bo binary.ByteOrder, implicit IsImplicitVR) {
	return d.bo, d.implicit
}

// Temporarily change the encoding format. PopTrasnferSyntax() will restore the
// old format.
func (d *Decoder) PushTransferSyntax(bo binary.ByteOrder, implicit IsImplicitVR) {
	d.oldTransferSyntaxes = append(d.oldTransferSyntaxes, transferSyntaxStackEntry{d.bo, d.implicit})
	d.bo = bo
	d.implicit = implicit
}

// Override the default (7bit ASCII) decoder used when converting a byte[] to a
// string.
func (d *Decoder) SetCodingSystem(cs CodingSystem) {
	d.codingSystem = cs
}

// Restore the encoding format active before the last call to
// PushTransferSyntax().
func (d *Decoder) PopTransferSyntax() {
	e := d.oldTransferSyntaxes[len(d.oldTransferSyntaxes)-1]
	d.bo = e.bo
	d.implicit = e.implicit
	d.oldTransferSyntaxes = d.oldTransferSyntaxes[:len(d.oldTransferSyntaxes)-1]
}

// Temporarily override the end of the buffer. PopLimit() will restore the old
// limit.
//
// REQUIRES: limit must be smaller than the current limit
func (d *Decoder) PushLimit(bytes int64) {
	newLimit := d.pos + bytes
	if newLimit > d.limit {
		d.SetError(fmt.Errorf("Trying to read %d bytes beyond buffer end", newLimit-d.limit))
		newLimit = d.pos
	}
	d.oldLimits = append(d.oldLimits, d.limit)
	d.limit = newLimit
}

// Restore the old limit overridden by PushLimit.
func (d *Decoder) PopLimit() {
	d.limit = d.oldLimits[len(d.oldLimits)-1]
	d.oldLimits = d.oldLimits[:len(d.oldLimits)-1]
}

// Returns an error encountered so far.
func (d *Decoder) Error() error { return d.err }

// Finish() must be called after using the decoder. It returns any error
// encountered during decoding. It also returns an error if some data is left
// unconsumed.
func (d *Decoder) Finish() error {
	if d.err != nil {
		return d.err
	}
	if d.Len() != 0 {
		return fmt.Errorf("Decoder found junk (%d bytes remaining)", d.Len())
	}
	return nil
}

// io.Reader implementation
func (d *Decoder) Read(p []byte) (int, error) {
	desired := d.Len()
	if desired == 0 {
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	if desired < int64(len(p)) {
		p = p[:desired]
		desired = int64(len(p))
	}
	n, err := d.in.Read(p)
	if err == nil {
		d.pos += int64(n)
	}
	return n, err
}

// Len() returns the number of bytes yet consumed.
func (d *Decoder) Len() int64 {
	return d.limit - d.pos
}

// DecodeByte() reads a single byte from the buffer. On EOF, it returns a junk
// value, and sets an error to be returned by Error() or Finish().
func (d *Decoder) DecodeByte() (v byte) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
		return 0
	}
	return v
}

func (d *Decoder) DecodeUInt32() (v uint32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) DecodeInt32() (v int32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) DecodeUInt16() (v uint16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) DecodeInt16() (v int16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) DecodeFloat32() (v float32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) DecodeFloat64() (v float64) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func internalDecodeString(d *Decoder, sd *encoding.Decoder, length int) string {

	bytes := d.DecodeBytes(length)
	if len(bytes) == 0 {
		return ""
	}
	if sd == nil {
		// Assume that UTF-8 is a superset of ASCII.
		// TODO(saito) check that string is 7-bit clean.
		return string(bytes)
	}
	bytes, err := sd.Bytes(bytes)
	if err != nil {
		d.SetError(err)
		return ""
	}
	return string(bytes)
}

func (d *Decoder) DecodeStringWithCodingSystem(csType CodingSystemType, length int) string {
	var sd *encoding.Decoder
	switch csType {
	case AlphabeticCodingSystem:
		sd = d.codingSystem.Alphabetic
	case IdeographicCodingSystem:
		sd = d.codingSystem.Ideographic
	case PhoneticCodingSystem:
		sd = d.codingSystem.Phonetic
	default:
		panic(csType)
	}
	return internalDecodeString(d, sd, length)
}

func (d *Decoder) DecodeString(length int) string {
	return internalDecodeString(d, d.codingSystem.Ideographic, length)
}

func (d *Decoder) DecodeBytes(length int) []byte {
	if d.Len() < int64(length) {
		d.SetError(fmt.Errorf("DecodeBytes: requested %d, available %d",
			length, d.Len()))
		return nil
	}
	v := make([]byte, length)
	remaining := v
	for len(remaining) > 0 {
		n, err := d.Read(v)
		if err != nil {
			d.SetError(err)
			break
		}
		remaining = remaining[n:]
	}
	doassert(d.Error() != nil || len(remaining) == 0)
	return v
}

func (d *Decoder) Skip(length int) {
	if d.Len() < int64(length) {
		d.SetError(fmt.Errorf("Skip: requested %d, available %d",
			length, d.Len()))
		return
	}
	junkSize := 1 << 16
	if length < junkSize {
		junkSize = length
	}
	junk := make([]byte, junkSize)
	remaining := length
	for remaining > 0 {
		tmpLength := len(junk)
		if remaining < tmpLength {
			tmpLength = remaining
		}
		tmpBuf := junk[:tmpLength]
		n, err := d.Read(tmpBuf)
		if err != nil {
			d.SetError(err)
			break
		}
		doassert(n > 0)
		remaining -= n
	}
	doassert(d.Error() != nil || remaining == 0)
}
