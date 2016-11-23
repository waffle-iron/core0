package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"

	"github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger("crypto")
)

func CreateSessionKey(hash string) []byte {
	return []byte(hash[:32])
}

func ReadPrivateKey(b []byte) (*rsa.PrivateKey, error) {

	// Extract the PEM-encoded data block
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("bad key data: %s", "not PEM-encoded")
	}

	// Decode the private key
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return priv, nil
}

func EncryptAsym(key *rsa.PublicKey, msg []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, key, msg)
}

func DecryptAsym(key *rsa.PrivateKey, msg []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, key, msg)
}

func EncryptSymStream(key []byte, in io.Reader) (io.Reader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	//create initial vector
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	pReader, pWriter := io.Pipe()
	go func() {
		//put initial vector on output
		defer func() {
			if closer, ok := in.(io.Closer); ok {
				closer.Close()
			}
		}()

		buff := bytes.NewBuffer(iv)
		if _, err := buff.WriteTo(pWriter); err != nil {
			pWriter.CloseWithError(err)
			return
		}

		//stream encrypt
		stream := cipher.NewCFBEncrypter(block, iv)
		writer := &cipher.StreamWriter{S: stream, W: pWriter}

		// Copy the input file to the output file, encrypting as we go.
		if _, err := io.Copy(writer, in); err != nil {
			pWriter.CloseWithError(err)
			return
		}

		pWriter.Close()
	}()

	return pReader, nil
}

func EncryptSym(key []byte, in io.Reader, out io.Writer) error {
	reader, err := EncryptSymStream(key, in)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}

	return nil
}

func DecryptSym(key []byte, in io.Reader, out io.Writer) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// read initial vector from input
	iv := make([]byte, aes.BlockSize)
	if n, err := io.ReadFull(in, iv); err != nil {
		log.Errorf("Error readFull %v: %v", n, err)
		return err
	}

	//stream decrypt
	stream := cipher.NewCFBDecrypter(block, iv)
	reader := &cipher.StreamReader{S: stream, R: in}
	// Copy the input file to the output file, decrypting as we go.
	if _, err := io.Copy(out, reader); err != nil {
		return err
	}

	return nil
}
