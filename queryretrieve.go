package dicom

import (
	"fmt"
	"v.io/x/lib/vlog"
)

func querySequence(elem *Element, f *Element) (match bool, err error) {
	// TODO(saito) Implement!
	return true, nil
}

func queryElement(elem *Element, f *Element) (match bool, err error) {
	if len(f.Value) == 0 {
		// Universal match
		return true, nil
	}
	if f.VR == "SQ" {
		return querySequence(f, elem)
	}
	if elem == nil {
		return false, err
	}
	if f.VR != elem.VR {
		// This shouldn't happen, but be paranoid
		return false, fmt.Errorf("VR mismatch: filter %v, value %v", f, elem)
	}
	if f.VR == "UI" {
		// See if elem contains at last one uid listed in the filter.
		for _, expected := range f.Value {
			e := expected.(string)
			for _, value := range elem.Value {
				if value.(string) == e {
					return true, nil
				}
			}
		}
		return false, nil
	}
	if len(f.Value) > 1 {
		// A filter can't contain multiple values. Ps3.4, C.2.2.2.1
		return false, fmt.Errorf("Multiple values found in filter '%v'", f)
	}
	// TODO: handle date-range matches
	switch v := f.Value[0].(type) {
	case int32:
		for _, value := range elem.Value {
			if v == value.(int32) {
				return true, nil
			}
		}
	case int16:
		for _, value := range elem.Value {
			if v == value.(int16) {
				return true, nil
			}
		}
	case uint32:
		for _, value := range elem.Value {
			if v == value.(uint32) {
				return true, nil
			}
		}
	case uint16:
		for _, value := range elem.Value {
			if v == value.(uint16) {
				return true, nil
			}
		}
	case float32:
		for _, value := range elem.Value {
			if v == value.(float32) {
				return true, nil
			}
		}
	case float64:
		for _, value := range elem.Value {
			if v == value.(float64) {
				return true, nil
			}
		}

	case string:
		for _, value := range elem.Value {
			if v == value.(string) {
				return true, nil
			}
		}
	default:
		vlog.Fatalf("Unknown data: %v", f)
	}
	return false, nil
}

func Query(ds *DataSet, f *Element) (match bool, matchedElem *Element, err error) {
	if f.Tag == TagQueryRetrieveLevel || f.Tag == TagSpecificCharacterSet {
		return true, nil, nil
	}
	elem, err := ds.LookupElementByTag(f.Tag)
	if err != nil {
		elem = nil
	}

	match, err = queryElement(elem, f)
	if match {
		return true, elem, nil
	}
	return false, nil, err
}
