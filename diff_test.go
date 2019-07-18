package binarydist

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

func testFunc(t *testing.T, fDiff func(old, new io.Reader, patch io.Writer) error) {
	t.Helper()

	diffT := []struct {
		old *os.File
		new *os.File
	}{
		{
			old: mustWriteRandFile("test.old", 1e3, 1),
			new: mustWriteRandFile("test.new", 1e3, 2),
		},
		{
			old: mustOpen("testdata/sample.old"),
			new: mustOpen("testdata/sample.new"),
		},
	}

	for _, s := range diffT {
		got, err := ioutil.TempFile("/tmp", "bspatch.")
		if err != nil {
			panic(err)
		}
		os.Remove(got.Name())

		exp, err := ioutil.TempFile("/tmp", "bspatch.")
		if err != nil {
			panic(err)
		}

		cmd := exec.Command("bsdiff", s.old.Name(), s.new.Name(), exp.Name())
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		os.Remove(exp.Name())
		if err != nil {
			panic(err)
		}

		err = fDiff(s.old, s.new, got)
		if err != nil {
			t.Fatal("err", err)
		}

		_, err = got.Seek(0, 0)
		if err != nil {
			panic(err)
		}
		gotBuf := mustReadAll(got)
		expBuf := mustReadAll(exp)

		if !bytes.Equal(gotBuf, expBuf) {
			t.Fail()
			t.Logf("diff %s %s", s.old.Name(), s.new.Name())
			t.Logf("%s: len(got) = %d", got.Name(), len(gotBuf))
			t.Logf("%s: len(exp) = %d", exp.Name(), len(expBuf))
			i := matchlen(gotBuf, expBuf)
			t.Logf("produced different output at pos %d; %d != %d", i, gotBuf[i], expBuf[i])
		}
	}
}

func TestDiff(t *testing.T) {
	testFunc(t, Diff)
}

func TestDiffWithSuf(t *testing.T) {
	fDiff := func(old, new io.Reader, patch io.Writer) error {
		oldSufStruct, err := ComputeSuf(old)
		if err != nil {
			return err
		}

		return DiffWithSuf(oldSufStruct, new, patch)
	}
	testFunc(t, fDiff)
}
