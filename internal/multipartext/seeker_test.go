package multipartext

import (
	"bytes"
	"io"
	"testing"
)

func Test_multiReadSeeker_WriteTo(t *testing.T) {
	type fields struct {
		readers []SizedReadSeeker
		index   int
		size    int64
	}
	tests := []struct {
		name    string
		fields  fields
		startAt int64
		wantW   string
		wantSum int64
		wantErr bool
	}{
		{
			name: "read from the start",
			fields: fields{
				readers: []SizedReadSeeker{
					{bytes.NewReader([]byte("hello")), 5},
					{bytes.NewReader([]byte(" world")), 6},
				},
				size: 11,
			},
			startAt: 0,
			wantW:   "hello world",
			wantSum: 11,
			wantErr: false,
		},
		{
			name: "read from the middle",
			fields: fields{
				readers: []SizedReadSeeker{
					{bytes.NewReader([]byte("hello")), 5},
					{bytes.NewReader([]byte(" world")), 6},
					{bytes.NewReader([]byte("!")), 1},
				},
				size: 12,
			},
			startAt: 6,
			wantW:   "world!",
			wantSum: 6,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := &multiReadSeeker{
				readers: tt.fields.readers,
				size:    tt.fields.size,
			}

			// Run it twice to make sure it's repeatable, as that's the whole
			// point of a ReadSeeker.
			for i := 0; i < 2; i++ {
				_, err := mr.Seek(tt.startAt, io.SeekStart)
				if err != nil {
					t.Errorf("Seek() error = %v", err)
					return
				}
				w := &bytes.Buffer{}
				gotSum, err := mr.WriteTo(w)
				if (err != nil) != tt.wantErr {
					t.Errorf("WriteTo() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if gotW := w.String(); gotW != tt.wantW {
					t.Errorf("WriteTo() gotW = %v, want %v", gotW, tt.wantW)
				}
				if gotSum != tt.wantSum {
					t.Errorf("WriteTo() gotSum = %v, want %v", gotSum, tt.wantSum)
				}
			}
		})
	}
}

func Test_multiReadSeeker_Read(t *testing.T) {
	type fields struct {
		readers []SizedReadSeeker
		index   int
		offset  int64
		size    int64
	}
	type args struct {
		p []byte
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		startAt   int64
		readTimes int
		wantN     int
		wantErr   bool
	}{
		{
			name: "read all from the start",
			fields: fields{
				readers: []SizedReadSeeker{
					{bytes.NewReader([]byte("hello")), 5},
					{bytes.NewReader([]byte(" world")), 6},
					{bytes.NewReader([]byte("!")), 1},
				},
				size: 12,
			},
			args: args{
				p: make([]byte, 32),
			},
			readTimes: 3,
			startAt:   0,
			wantN:     12,
			wantErr:   false,
		},
		{
			name: "read from the middle",
			fields: fields{
				readers: []SizedReadSeeker{
					{bytes.NewReader([]byte("hello")), 5},
					{bytes.NewReader([]byte(" world")), 6},
					{bytes.NewReader([]byte("!")), 1},
				},
				size: 12,
			},
			args: args{
				p: make([]byte, 32),
			},
			readTimes: 2,
			startAt:   6,
			wantN:     6,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := &multiReadSeeker{
				readers: tt.fields.readers,
				offset:  tt.fields.offset,
				size:    tt.fields.size,
			}

			// Run it twice to make sure it's repeatable, as that's the whole
			// point of a ReadSeeker.
			for i := 0; i < 2; i++ {
				_, err := mr.Seek(tt.startAt, io.SeekStart)
				if err != nil {
					t.Errorf("Seek() error = %v", err)
					return
				}

				var sum int
				for i := 0; i < tt.readTimes; i++ {
					var gotN int
					gotN, err = mr.Read(tt.args.p)
					sum += gotN
				}
				if (err != nil) != tt.wantErr {
					t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if sum != tt.wantN {
					t.Errorf("Read() gotN = %v, want %v", sum, tt.wantN)
				}
			}
		})
	}
}
