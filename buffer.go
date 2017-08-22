package dicom

import (
	"fmt"
	"io"
	"encoding/binary"
)

type Decoder struct {
	in        io.Reader
	err       error

	bo       binary.ByteOrder
	implicit bool

	// Cumulative # bytes read.
	pos    int
	// Max bytes to read. PushLimit() will add a new limit, and PopLimit()
	// will restore the old limit. The newest limit is at the end.
	//
	// INVARIANT: limits[] store values in decreasing order.
	limits []int
}

func NewDecoder(
	in io.Reader,
	limit int,
	bo binary.ByteOrder,
	implicit bool) *Decoder {
	return &Decoder{
		in: in,
		err: nil,
		bo: bo,
		implicit: implicit,
		pos: 0,
		limits: []int{limit},
	}
}

func (d *Decoder) PushLimit(limit int) {
	d.limits = append(d.limits, d.pos + limit)
}

func (d *Decoder) PopLimit() {
	d.limits = d.limits[:len(d.limits)-1]
}

func (d *Decoder) Pos() int { return d.pos }

func (d *Decoder) Error() error { return d.err }

func (d *Decoder) Finish() error {
	if d.err != nil {
		return d.err
	}
	if d.Available() != 0 {
		return fmt.Errorf("Decoder found junk (%d bytes remaining)", d.Available())
	}
	return nil
}

// io.Reader implementation
func (d *Decoder) Read(p []byte) (int, error) {
	desired := d.Available()
	if desired == 0 {
		if len(p)==0 {return 0, nil}
		return 0, io.EOF
	}
	if desired < len(p) {
		p = p[:desired]
		desired = len(p)
	}
	n, err := d.in.Read(p)
	if err == nil {
		d.pos += n
	}
	return n, err
}

func (d *Decoder) Available() int {
	return d.limits[len(d.limits)-1] - d.pos
}

func (d *Decoder) Len() int { return d.Available() }

func (d *Decoder) DecodeByte() (v byte) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
		return 0
	}
	return v
}

func (d *Decoder) DecodeUInt32() (v uint32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeInt32() (v int32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeUInt16() (v uint16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeInt16() (v int16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeFloat32() (v float32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeFloat64() (v float64) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeString(length int) string {
	return string(d.DecodeBytes(length))
}

func (d *Decoder) DecodeBytes(length int) []byte {
	v := make([]byte, length)
	remaining := v
	for len(remaining) > 0 {
		n, err := d.Read(v)
		if err != nil {
			d.err = err
			break
		}
		remaining = remaining[n:]
	}
	//doassert(d.err==nil)
	if len(remaining) > 0 {
		d.err = fmt.Errorf("DecodeBytes: requested %d, remaining %d",
			length, len(remaining))
		panic(d.err)  // TODO(saito) remove
	}
	return v
}

func (d *Decoder) Skip(bytes int) {
	junk := make([]byte, bytes)
	n, err := d.Read(junk)
	if err != nil {
		d.err = err
		return
	}
	if n != bytes {
		d.err = fmt.Errorf("Failed to skip %d bytes (read %d bytes instead)", bytes, n)
		return
	}
}
