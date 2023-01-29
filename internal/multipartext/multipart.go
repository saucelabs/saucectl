package multipartext

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
)

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// NewMultipartReader creates a new io.Reader that serves multipart form-data from src.
// Also returns the form data content type (see multipart.Writer#FormDataContentType).
func NewMultipartReader(filename, description string, src io.Reader) (io.Reader, string, error) {
	// Create the multipart header.
	buffy := &bytes.Buffer{}
	writer := multipart.NewWriter(buffy)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="payload"; filename="%s"`, quoteEscaper.Replace(filename)))
	header.Set("Content-Type", "application/octet-stream")

	// Create the actual part that will hold the data. Though we won't actually write the data just yet, since we want
	// to stream it later.
	if _, err := writer.CreatePart(header); err != nil {
		return nil, "", err
	}
	headerSize := buffy.Len()

	if err := writer.WriteField("description", description); err != nil {
		return nil, "", err
	}

	// Finish the multipart message.
	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return io.MultiReader(
			bytes.NewReader(buffy.Bytes()[:headerSize]),
			src,
			bytes.NewReader(buffy.Bytes()[headerSize:]),
		),
		writer.FormDataContentType(),
		nil
}
