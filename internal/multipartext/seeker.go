package multipartext

import (
	"fmt"
	"io"
)

// SizedReadSeeker is a ReadSeeker that also knows its size.
type SizedReadSeeker struct {
	reader io.ReadSeeker
	size   int64
}

func MultiReadSeeker(readers ...io.ReadSeeker) (io.ReadSeeker, error) {
	var sizedReaders []SizedReadSeeker

	var sumSize int64
	for _, r := range readers {
		// get the size
		n, err := r.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, err
		}
		sumSize += n

		// now reset
		_, _ = r.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}

		sizedReaders = append(sizedReaders, SizedReadSeeker{reader: r, size: n})
	}
	return &multiReadSeeker{readers: sizedReaders, size: sumSize}, nil
}

// multiReadSeeker is a ReadSeeker that reads from multiple SizedReadSeekers.
type multiReadSeeker struct {
	readers []SizedReadSeeker
	offset  int64
	size    int64
}

func (mr *multiReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += mr.offset
	case io.SeekEnd:
		offset += mr.size
	default:
		return 0, fmt.Errorf("seek: invalid whence %d", whence)
	}

	if offset > mr.size {
		return 0, fmt.Errorf("seek: invalid offset %d larger than size %d", offset, mr.size)
	}

	if offset < 0 {
		return 0, fmt.Errorf("seek: invalid negative offset %d", offset)
	}

	mr.offset = offset
	return offset, nil
}

func (mr *multiReadSeeker) Read(p []byte) (n int, err error) {
	offset := mr.offset
	for i, r := range mr.readers {
		if r.size-offset < 0 {
			offset -= r.size
			continue
		}

		_, err = r.reader.Seek(offset, io.SeekStart)
		if err != nil {
			return 0, err
		}

		n, err = r.reader.Read(p)
		mr.offset += int64(n)
		offset -= int64(n)
		if n == 0 {
			offset -= r.size
		}
		if offset < 0 {
			offset = 0
		}

		if n > 0 || err != io.EOF {
			if err == io.EOF && i < len(mr.readers)-1 {
				// Don't return EOF yet. More readers remain.
				err = nil
			}
			return
		}
	}

	return 0, io.EOF
}

func (mr *multiReadSeeker) WriteTo(w io.Writer) (sum int64, err error) {
	buf := make([]byte, 1024*32)

	offset := mr.offset
	for _, r := range mr.readers {
		if r.size-offset < 0 {
			offset -= r.size
			continue
		}

		_, err = r.reader.Seek(offset, io.SeekStart)
		if err != nil {
			return sum, err
		}

		n, err := io.CopyBuffer(w, r.reader, buf)
		sum += n
		mr.offset += n
		offset -= n
		if offset < 0 {
			offset = 0
		}
		if err != nil {
			return sum, err // permit resume / retry after error
		}
	}
	return sum, nil
}
