// package parget provides message type and helper functions
// to work with parallel download
package parget

import (
	"encoding/gob"
	"errors"
	"io"
)

const (
	TypGetLen = iota
	TypGet
	TypLen
	TypData
	TypErr
)

// type Msg describes universal message format
// used by all communications (including data tranfer)
type Msg struct {
	Typ         byte // Type of message
	Offset, Len int64
	Path        string // Path to file
	Data        []byte // chunk of file
}

// Read gob-decodes incoming Msg from the Reader
func Read(in io.Reader) (*Msg, error) {
	if in == nil {
		return nil, errors.New("Reader is nil")
	}

	res := &Msg{}
	err := gob.NewDecoder(in).Decode(res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Write gob-encodes Msg into a Writer
func Write(out io.Writer, m Msg) error {
	if out == nil {
		return errors.New("Writer is nil")
	}

	return gob.NewEncoder(out).Encode(m)
}
