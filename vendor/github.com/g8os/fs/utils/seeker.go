package utils

import (
	"bytes"
	"io"
	"io/ioutil"
)

type bodyReader struct {
	body   io.ReadCloser
	reader *bytes.Reader
}

func NewReadSeeker(body io.ReadCloser) io.ReadSeeker {
	return &bodyReader{
		body: body,
	}
}

func (reader *bodyReader) fillBuffer() error {
	if reader.reader == nil {
		content, err := ioutil.ReadAll(reader.body)
		if err != nil {
			return err
		}
		reader.reader = bytes.NewReader(content)
	}
	return nil
}

func (reader *bodyReader) Read(p []byte) (int, error) {
	if err := reader.fillBuffer(); err != nil {
		return 0, nil
	}
	return reader.reader.Read(p)
}

func (reader *bodyReader) Close() error {
	return reader.body.Close()
}

func (reader *bodyReader) Seek(offset int64, whence int) (int64, error) {
	if err := reader.fillBuffer(); err != nil {
		return 0, nil
	}
	return reader.reader.Seek(offset, whence)
}
