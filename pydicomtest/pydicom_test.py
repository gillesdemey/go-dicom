#!/usr/bin/env python3.6

import sys
import os

sys.path.append(os.path.join(os.environ['HOME'], 'pydicom'))
sys.path.append(os.path.join(os.environ['HOME'], 'pynetdicom3'))
import pydicom


def recurse_tree(dataset, nest_level: int):
    # order the dicom tags
    for data_element in dataset:
        indent = "  " * nest_level
        print(f"{indent}{data_element.tag} {data_element.VR}: ", end="")
        if data_element.VR in ("OW", "OB", "OD", "OF", "LT", "LO"): # long text
            print(f"{len(data_element.value)}bytes")
        elif data_element.VR != "SQ":   # not a sequence
            print(str(data_element.value))
        else:   # a sequence
            print("")
            for i, child in enumerate(data_element.value):
                recurse_tree(child, nest_level + 1)

def main():
    ds = pydicom.read_file(sys.argv[1])
    ds.decode()
    recurse_tree(ds, 0)

main()
