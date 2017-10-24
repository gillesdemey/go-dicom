# DICOM parser in Go [![GoDoc](https://godoc.org/github.com/gillesdemey/go-dicom?status.svg)](https://godoc.org/github.com/gillesdemey/go-dicom) [![wercker status](https://app.wercker.com/status/c250d72bc82a5d8f267c7ee0b9e839bc/s/master "wercker status")](https://app.wercker.com/project/bykey/c250d72bc82a5d8f267c7ee0b9e839bc)

This is a fork of github.com/gillesdemey/go-dicom. Changes are:

- Many bug fixes, especially around handling of sequences.
- Handle non-ASCII characters more properly.
- Simplify the API. All the functions are synchronous.
- Better library supports around tags & uids.
- Rudimentary support for writing DICOM files. This part is not complete yet.
- Adds fuzz tests and tests that ensure compatibility with pydicom.

TODO:
- Implement mixed-coding-system files more properly. We currently botch
  patient-name (PN) elements that mixes coding systems.

- A multi-image file. Functionality is almost there, but I haven't had time to complete it.

- Native pixeldata format. It'll be parsed as just []byte.


See doc.go for usage. dicomutil contains a sample program that dumps DICOM
elements in a file.


### Acknowledgements

I'd like to thank my friend [Seppe Stas](https://github.com/Bitbored/) for helping me get through the horrific DICOM image specification and some of the harder parts of the parser.

Some more inspiration and helpful resource that brought this library to life (in no particular order):

DWV by ivmartel https://github.com/ivmartel/dwv/ <br>
dicomParser by Chris Hafey https://github.com/chafey/dicomParser <br>
http://www.dicomlibrary.com <br>
http://dicom.nema.org/medical/dicom/current/output/pdf/part05.pdf <br>
