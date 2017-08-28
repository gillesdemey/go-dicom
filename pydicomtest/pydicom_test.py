#!/usr/bin/env python3.6

import io
import logging
import os
import re
import subprocess
import sys
import tempfile

#sys.path.append(os.path.join(os.environ['HOME'], 'pydicom'))
#sys.path.append(os.path.join(os.environ['HOME'], 'pynetdicom3'))
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
    tmp = io.StringIO()
    recurse_tree(ds, tmp, 0)

    # Remove single quotes around string. I can't tell any rule about quotes are
    # added, so just strip them unconditionalyl.
    with open(out_path, 'w') as out:
        out.write(tmp.getvalue().replace("'", ''))

def print_file_using_godicom(dicom_path: str, out_path: str):
    with tempfile.TemporaryFile(mode='w+b') as tmp:
        subprocess.check_call(['./pydicomtest', dicom_path],
                              stdout=tmp)
        tmp.seek(0)
        with open(out_path, 'wb') as out:
            out.write(tmp.read().replace(b"'", b''))

def process_one_file(dicom_path: str):
    if (dicom_path.endswith('ExplVR_BigEndNoMeta.dcm') or
        dicom_path.endswith('ExplVR_LitEndNoMeta.dcm') or
        dicom_path.endswith('OT-PAL-8-face.dcm')):
        logging.info("Skip %s, it is known to be broken", dicom_path)
        return

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
    subprocess.check_call(['go', 'build', '.'])
    if os.path.isdir(dicom_path):
        for dirpath, dirnames, filenames in os.walk(dicom_path):
            for filename in filenames:
                if filename.endswith(".dcm"):
                    process_one_file(os.path.join(dirpath, filename))
    else:
        process_one_file(dicom_path)

    logging.info("All files passed the test!")
main()
