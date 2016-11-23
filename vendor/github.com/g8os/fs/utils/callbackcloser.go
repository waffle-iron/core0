package utils

import "io"

type OnClose func(string, io.ReadSeeker)

type callbackCloser struct {
	io.ReadSeeker
	path     string
	callback OnClose
}

func NewCallbackCloser(f io.ReadSeeker, path string, cb OnClose) io.ReadSeeker {
	return &callbackCloser{
		ReadSeeker: f,
		path:       path,
		callback:   cb}
}

func (r *callbackCloser) Close() error {
	r.callback(r.path, r.ReadSeeker)
	if closer, ok := r.ReadSeeker.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
