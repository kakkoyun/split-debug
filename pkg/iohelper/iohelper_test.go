package iohelper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// AtToWriter convert a WriterAt to a Writer
func AtToWriter(w io.WriterAt, offset int64) io.Writer {
	return NewSectionWriter(w, offset, maxOffset-offset)
}

// AtToReader convert a ReaderAt to a Reader
func AtToReader(r io.ReaderAt, offset int64) io.Reader {
	return io.NewSectionReader(r, offset, maxOffset-offset)
}

// Tests except TestSectionWriter_Write are copied from `io_test.go`.
// Surprised that `io_test.go` does not test SectionReader.Write.

var errBufferTooShort = errors.New("buffer is too short")

// `fooWriterAt` is a simple WriterAt implementation for test.
// It returns errBufferTooShort if trying to write out of underlaying buffer
// boundary.
type fooWriterAt struct {
	Buf []byte
}

func NewFooWriterAt(l int) *fooWriterAt {
	return &fooWriterAt{
		Buf: make([]byte, l),
	}
}

func (rw *fooWriterAt) WriteAt(p []byte, offset int64) (n int, err error) {
	total := int(offset) + len(p)
	n = len(p)
	if total > len(rw.Buf) {
		n -= total - len(rw.Buf)
		if n < 0 {
			n = 0
		}
		err = errBufferTooShort
	}
	for i := 0; i < n; i++ {
		rw.Buf[int(offset)+i] = p[i]
	}
	return
}

func clamp(v, l, r int) int {
	if v < l {
		v = l
	}

	if v > r {
		v = r
	}

	return v
}

func TestSectionWriter_WriteAt(t *testing.T) {
	ta := require.New(t)

	const dat = "a long sample data, 1234567890"
	tests := []struct {
		data   string
		off    int
		n      int
		bufLen int
		at     int
		exp    string
		err    error
	}{
		{data: "", off: 0, n: 10, bufLen: 2, at: 0, exp: "", err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: 0, at: 0, exp: "", err: errBufferTooShort},
		{data: dat, off: len(dat), n: 1, bufLen: 1, at: 0, exp: "", err: errBufferTooShort},
		{data: dat, off: 0, n: len(dat) + 2, bufLen: len(dat), at: 0, exp: dat, err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: len(dat) / 2, at: 0, exp: dat[:len(dat)/2], err: errBufferTooShort},
		// 5
		{data: dat, off: 0, n: len(dat), bufLen: len(dat), at: 0, exp: dat, err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: len(dat) / 2, at: 2, exp: dat[:len(dat)/2-2], err: errBufferTooShort},
		{data: dat, off: 3, n: len(dat), bufLen: len(dat) / 2, at: 2, exp: dat[:len(dat)/2-5], err: errBufferTooShort},
		{data: dat, off: 3, n: len(dat) / 2, bufLen: len(dat)/2 - 2, at: 2, exp: dat[:len(dat)/2-7], err: errBufferTooShort},
		{data: dat, off: 3, n: len(dat) / 2, bufLen: len(dat)/2 + 2, at: 2, exp: dat[:len(dat)/2-3], err: errBufferTooShort},
		// 10
		{data: dat, off: 0, n: 0, bufLen: 0, at: -1, exp: "", err: io.ErrShortWrite},
		{data: dat, off: 0, n: 0, bufLen: 0, at: 1, exp: "", err: io.ErrShortWrite},

		// Test ErrShortWrite returned from SectionWriter.WriteAt if exceeds
		// section boundary.
		{data: dat, off: 2, n: 5, bufLen: 10, at: 1, exp: "a lo", err: io.ErrShortWrite},
	}
	for i, tt := range tests {
		w := NewFooWriterAt(tt.bufLen)
		s := NewSectionWriter(w, int64(tt.off), int64(tt.n))
		buf := []byte(tt.data)
		n, err := s.WriteAt(buf, int64(tt.at))

		left := clamp(tt.off+tt.at, 0, len(w.Buf))
		right := clamp(tt.off+tt.at+n, 0, len(w.Buf))

		msg := fmt.Sprintf("%d: WriteAt(%d) = %q, %v; expected %q, %v",
			i, tt.at, w.Buf[left:right], err, tt.exp, tt.err)

		ta.Equal(len(tt.exp), n, msg)
		ta.Equal(tt.exp, string(w.Buf[left:right]), msg)
		ta.Equal(tt.err, err, msg)
	}
}

func TestSectionWriter_Write(t *testing.T) {
	ta := require.New(t)

	const dat = "a long sample data, 1234567890"
	tests := []struct {
		data   string
		off    int
		n      int
		bufLen int
		exp    string
		err    error
	}{
		{data: "", off: 0, n: 10, bufLen: 2, exp: "", err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: 0, exp: "", err: errBufferTooShort},
		{data: dat, off: len(dat), n: 1, bufLen: 1, exp: "", err: errBufferTooShort},
		{data: dat, off: 0, n: len(dat) + 2, bufLen: len(dat), exp: dat, err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: len(dat) / 2, exp: dat[:len(dat)/2], err: errBufferTooShort},
		{data: dat, off: 0, n: len(dat), bufLen: len(dat), exp: dat, err: nil},
		{data: dat, off: 0, n: len(dat) / 2, bufLen: len(dat), exp: dat[:len(dat)/2], err: io.ErrShortWrite},
		{data: dat, off: len(dat) / 2, n: len(dat) / 2, bufLen: len(dat), exp: dat[:len(dat)/2], err: io.ErrShortWrite},
	}
	for i, tt := range tests {
		w := NewFooWriterAt(tt.bufLen)
		s := NewSectionWriter(w, int64(tt.off), int64(tt.n))
		buf := []byte(tt.data)
		n, err := s.Write(buf)

		left := clamp(tt.off, 0, len(w.Buf))
		right := clamp(tt.off+n, 0, len(w.Buf))

		msg := fmt.Sprintf("%d: Write() = %q, %v; expected %q, %v", i, w.Buf[left:right], err, tt.exp, tt.err)

		ta.Equal(len(tt.exp), n, msg)
		ta.Equal(tt.exp, string(w.Buf[left:right]), msg)
		ta.Equal(tt.err, err, msg)
	}
}

func TestSectionWriter_Seek(t *testing.T) {
	ta := require.New(t)

	// Verifies that NewSectionWriter's Seeker behaves like bytes.NewReader (which is like strings.NewReader)
	br := bytes.NewReader([]byte("foo"))
	w := NewFooWriterAt(3)
	sw := NewSectionWriter(w, 0, int64(len("foo")))

	for _, whence := range []int{io.SeekStart, io.SeekCurrent, io.SeekEnd} {
		for offset := int64(-3); offset <= 4; offset++ {
			brOff, brErr := br.Seek(offset, whence)
			srOff, srErr := sw.Seek(offset, whence)
			if (brErr != nil) != (srErr != nil) || brOff != srOff {
				t.Errorf("For whence %d, offset %d: bytes.Writer.Seek = (%v, %v) != SectionReader.Seek = (%v, %v)",
					whence, offset, brOff, brErr, srErr, srOff)
			}
		}
	}

	// And verify we can just seek past the end and get an io.EOF
	got, err := sw.Seek(100, io.SeekStart)
	if err != nil || got != 100 {
		t.Errorf("Seek = %v, %v; want 100, nil", got, err)
	}

	_, err = sw.Seek(100, 3)
	ta.Equal("Seek: invalid whence", err.Error(), "invalid whence")

	_, err = sw.Seek(-100, io.SeekStart)
	ta.Equal("Seek: invalid offset", err.Error(), "invalid offset")

	n, err := sw.Write(make([]byte, 10))
	if n != 0 || err != io.ErrShortWrite {
		t.Errorf("Write = %v, %v; want 0, io.ErrShortWrite", n, err)
	}
}

func TestSectionWriter_Size(t *testing.T) {
	ta := require.New(t)

	tests := []struct {
		data string
		want int64
	}{
		{"a long sample data, 1234567890", 30},
		{"", 0},
	}

	for _, tt := range tests {
		w := &fooWriterAt{}
		sw := NewSectionWriter(w, 0, int64(len(tt.data)))

		got := sw.Size()
		msg := fmt.Sprintf("Size = %v; want %v", got, tt.want)

		ta.Equal(tt.want, got, msg)
	}
}

func TestAtToWriter(t *testing.T) {
	ta := require.New(t)

	dat := "a long sample data, 1234567890"
	tests := []struct {
		data   string
		off    int
		bufLen int
		exp    string
		err    error
	}{
		{data: "", off: 0, bufLen: 2, exp: "", err: nil},
		{data: dat, off: 0, bufLen: 0, exp: "", err: errBufferTooShort},
		{data: dat, off: len(dat), bufLen: 1, exp: "", err: errBufferTooShort},
		{data: dat, off: 0, bufLen: len(dat), exp: dat, err: nil},
		{data: dat, off: 0, bufLen: len(dat) / 2, exp: dat[:len(dat)/2], err: errBufferTooShort},
		{data: dat, off: 0, bufLen: len(dat), exp: dat, err: nil},
		{data: dat, off: len(dat) / 2, bufLen: len(dat), exp: dat[:len(dat)/2], err: errBufferTooShort},
	}
	for i, tt := range tests {
		w := NewFooWriterAt(tt.bufLen)
		s := AtToWriter(w, int64(tt.off))
		buf := []byte(tt.data)
		n, err := s.Write(buf)

		left := clamp(tt.off, 0, len(w.Buf))
		right := clamp(tt.off+n, 0, len(w.Buf))

		msg := fmt.Sprintf("%d: Write() = %q, %v; expected %q, %v", i, w.Buf[left:right], err, tt.exp, tt.err)

		ta.Equal(len(tt.exp), n, msg)
		ta.Equal(tt.exp, string(w.Buf[left:right]), msg)
		ta.Equal(tt.err, err, msg)
	}
}

func TestAtToReader(t *testing.T) {
	dat := "a long sample data, 1234567890"
	tests := []struct {
		data   string
		off    int
		bufLen int
		exp    string
		err    error
	}{
		{data: "", off: 0, bufLen: 2, exp: "", err: io.EOF},
		{data: dat, off: 0, bufLen: 0, exp: "", err: nil},
		{data: dat, off: len(dat), bufLen: 1, exp: "", err: io.EOF},
		{data: dat, off: 0, bufLen: len(dat), exp: dat, err: nil},
		{data: dat, off: 0, bufLen: len(dat) / 2, exp: dat[:len(dat)/2], err: nil},
		{data: dat, off: 0, bufLen: len(dat), exp: dat, err: nil},
	}
	for i, tt := range tests {
		r := strings.NewReader(tt.data)
		s := AtToReader(r, int64(tt.off))
		buf := make([]byte, tt.bufLen)
		if n, err := s.Read(buf); n != len(tt.exp) || string(buf[:n]) != tt.exp || err != tt.err {
			t.Fatalf("%d: Read() = %q, %v; expected %q, %v", i, buf[:n], err, tt.exp, tt.err)
		}
	}
}
