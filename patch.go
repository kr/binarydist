// Package binarydist implements binary patch as described on
// http://www.daemonology.net/bsdiff/. It reads and writes files
// compatible with the tools there.
package binarydist

import (
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
)

var ErrCorrupt = errors.New("corrupt patch")

var magic = [8]byte{'B', 'S', 'D', 'I', 'F', 'F', '4', '0'}

// Patch applies patch to old, according to the bspatch algorithm,
// and writes the result to new.
func Patch(old io.Reader, new io.Writer, patch io.Reader) error {
	// File format:
	//   0       8    "BSDIFF40"
	//   8       8    X
	//   16      8    Y
	//   24      8    sizeof(newfile)
	//   32      X    bzip2(control block)
	//   32+X    Y    bzip2(diff block)
	//   32+X+Y  ???  bzip2(extra block)
	// with control block a set of triples (x,y,z) meaning "add x bytes
	// from oldfile to x bytes from the diff block; copy y bytes from the
	// extra block; seek forwards in oldfile by z bytes".

	var header struct {
		Magic   [8]byte
		CtrlLen int64
		DiffLen int64
		NewSize int64
	}
	err := binary.Read(patch, binary.LittleEndian, &header)
	if err != nil {
		return err
	}
	if header.Magic != magic {
		return ErrCorrupt
	}
	if header.CtrlLen < 0 || header.DiffLen < 0 || header.NewSize < 0 {
		return ErrCorrupt
	}

	ctrlbuf := make([]byte, header.CtrlLen)
	_, err = io.ReadFull(patch, ctrlbuf)
	if err != nil {
		return err
	}
	cpfbz2 := bzip2.NewReader(bytes.NewReader(ctrlbuf))

	diffbuf := make([]byte, header.DiffLen)
	_, err = io.ReadFull(patch, diffbuf)
	if err != nil {
		return err
	}
	dpfbz2 := bzip2.NewReader(bytes.NewReader(diffbuf))

	// The entire rest of the file is the extra block.
	epfbz2 := bzip2.NewReader(patch)

	obuf, err := ioutil.ReadAll(old)
	if err != nil {
		return err
	}

	nbuf := make([]byte, header.NewSize)

	var oldpos, newpos int64
	for newpos < header.NewSize {
		var ctrl struct{ Add, Copy, Seek int64 }
		err = binary.Read(cpfbz2, binary.LittleEndian, &ctrl)
		if err != nil {
			return err
		}

		// Sanity-check
		if newpos+ctrl.Add > header.NewSize {
			return ErrCorrupt
		}

		// Read diff string
		_, err = io.ReadFull(dpfbz2, nbuf[newpos:newpos+ctrl.Add])
		if err != nil {
			return ErrCorrupt
		}

		// Add old data to diff string
		for i := int64(0); i < ctrl.Add; i++ {
			if oldpos+i >= 0 && oldpos+i < int64(len(obuf)) {
				nbuf[newpos+i] += obuf[oldpos+i]
			}
		}

		// Adjust pointers
		newpos += ctrl.Add
		oldpos += ctrl.Add

		// Sanity-check
		if newpos+ctrl.Copy > header.NewSize {
			return ErrCorrupt
		}

		// Read extra string
		_, err = io.ReadFull(epfbz2, nbuf[newpos:newpos+ctrl.Copy])
		if err != nil {
			return ErrCorrupt
		}

		// Adjust pointers
		newpos += ctrl.Copy
		oldpos += ctrl.Seek
	}

	// Write the new file
	for len(nbuf) > 0 {
		n, err := new.Write(nbuf)
		if err != nil {
			return err
		}
		nbuf = nbuf[n:]
	}

	return nil
}
