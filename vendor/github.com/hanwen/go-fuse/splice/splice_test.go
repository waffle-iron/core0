package splice

import (
	"io/ioutil"
	"testing"
)

func TestPairSize(t *testing.T) {
	p, _ := Get()
	defer Done(p)
	p.MaxGrow()
	b := make([]byte, p.Cap()+100)
	for i := range b {
		b[i] = byte(i)
	}

	f, _ := ioutil.TempFile("", "splice")
	err := ioutil.WriteFile(f.Name(), b, 0644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = p.LoadFrom(f.Fd(), len(b))
	if err == nil {
		t.Fatalf("should give error on exceeding capacity")
	}

}
