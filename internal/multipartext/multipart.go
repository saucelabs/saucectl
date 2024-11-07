package multipartext

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/saucelabs/saucectl/internal/storage"
)

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// NewMultipartReader creates a new io.Reader that serves multipart form-data from src.
// Also returns the form data content type (see multipart.Writer#FormDataContentType).
func NewMultipartReader(field string, fileInfo storage.FileInfo, src io.Reader) (io.Reader, string, error) {
	// Create the multipart header.
	buffy := &bytes.Buffer{}
	writer := multipart.NewWriter(buffy)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition",
		fmt.Sprintf(
			`form-data; name="%s"; filename="%s"`,
			field,
			quoteEscaper.Replace(fileInfo.Name),
		),
	)
	header.Set("Content-Type", "application/octet-stream")

	// Create the actual part that will hold the data. Though we won't actually write the data just yet, since we want
	// to stream it later.
	if _, err := writer.CreatePart(header); err != nil {
		return nil, "", err
	}
	headerSize := buffy.Len()

	if err := writer.WriteField("description", fileInfo.Description); err != nil {
		return nil, "", err
	}

	if len(fileInfo.Tags) > 0 {
		if err := writer.WriteField("tags", strings.Join(fileInfo.Tags, ",")); err != nil {
			return nil, "", err
		}
	}

	// Finish the multipart message.
	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	if srcReadSeeker, ok := src.(io.ReadSeeker); ok {
		mrs, err := MultiReadSeeker(
			bytes.NewReader(buffy.Bytes()[:headerSize]),
			srcReadSeeker,
			bytes.NewReader(buffy.Bytes()[headerSize:]),
		)

		return mrs,
			writer.FormDataContentType(),
			err
	}

	return io.MultiReader(
			bytes.NewReader(buffy.Bytes()[:headerSize]),
			src,
			bytes.NewReader(buffy.Bytes()[headerSize:]),
		),
		writer.FormDataContentType(),
		nil
}
