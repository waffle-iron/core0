package utils

import (
	"crypto/md5"
	"fmt"
	"hash"
	"io"
)

type Hasher struct {
	m hash.Hash
}

func NewHasher(in io.Reader) (*Hasher, io.Reader) {
	m := md5.New()
	reader := io.TeeReader(in, m)
	return &Hasher{m}, reader
}

func (m *Hasher) Hash() string {
	return fmt.Sprintf("%x", m.m.Sum(nil))
}
