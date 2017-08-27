#!/usr/bin/env python3.6

import logging
import os
import subprocess
import sys

sys.path.append(os.path.join(os.environ['HOME'], 'pydicom'))
sys.path.append(os.path.join(os.environ['HOME'], 'pynetdicom3'))
import pydicom
from typing import IO

logging.basicConfig(level=logging.INFO)

def recurse_tree(dataset, out: IO[str], nest_level: int):
    # order the dicom tags
    for data_element in dataset:
        indent = "  " * nest_level
        print(f"{indent}{data_element.tag} {data_element.VR}:", end="", file=out)
        if data_element.tag.group == 0x7fe0 and data_element.tag.element == 0x10:
            print(' [omitted]', file=out)
        elif data_element.VR in ("LO", ):
            print(f" {data_element.value}", file=out)
        elif data_element.VR in ("OW", "OB", "OD", "OF", "LT", "LO"): # long text
            print(f" {len(data_element.value)}B", file=out)
        elif data_element.VR in ('FL', 'FD'):
            if type(data_element.value) is float:
                print(" %.4f" % data_element.value, file=out)
            else:
                print(" [" + ", ".join(["%.4f" % v for v in data_element.value]) + "]",
                      file=out)
        elif data_element.VR != "SQ":   # not a sequence
            v  = str(data_element.value)
            if len(v) > 0:
                print(" " + v, file=out)
            else:
                print("", file=out)
        else:   # a sequence
            print("", file=out)
            for i, child in enumerate(data_element.value):
                recurse_tree(child, out, nest_level + 1)

def print_file_using_pydicom(dicom_path: str, out_path: str):
    ds = pydicom.read_file(dicom_path)
    ds.decode()
    with open(out_path, 'w') as out:
        recurse_tree(ds, out, 0)

def print_file_using_godicom(dicom_path: str, out_path: str):
    with open(out_path, 'w') as out:
        subprocess.check_call(['./pydicomtest', dicom_path],
                              stdout=out)

def process_one_file(dicom_path: str):
    logging.info("Compare %s", dicom_path)
    print_file_using_pydicom(dicom_path, '/tmp/pydicom.txt')
    print_file_using_godicom(dicom_path, '/tmp/godicom.txt')

    # Ignore an item headers. Pydicom flattens all the Items in a sequence, so
    # the item headers aren't shown. On the other hand, godicom preserves the
    # hierarchy.
    if subprocess.call(['/usr/bin/diff', '-w',
                        '--ignore-matching-lines', 'fffe, e000',
                        '/tmp/pydicom.txt', '/tmp/godicom.txt']) != 0:
        logging.error("pydicom and godicom produced different results. Outputs are in /tmp/pydicom.txt and /tmp/godicom.txt")
        sys.exit(1)

def main():
    dicom_path = sys.argv[1]
    if os.path.isdir(dicom_path):
        for dirpath, dirnames, filenames in os.walk(dicom_path):
            for filename in filenames:
                if filename.endswith(".dcm"):
                    process_one_file(os.path.join(dirpath, filename))
    else:
        process_one_file(dicom_path)

main()
