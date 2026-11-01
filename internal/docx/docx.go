// Package docx extracts plain text from Office Open XML (.docx) documents
// using only the standard library. It reads word/document.xml and keeps
// paragraph, tab and line-break structure. Formatting, images, headers,
// footers, footnotes and tracked-change metadata are ignored.
package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// MaxUncompressed bounds the XML read from the archive (zip-bomb guard).
const MaxUncompressed = 30 << 20

var ErrNotDocx = errors.New("file is not a valid .docx document")

// ExtractText returns the document body text.
func ExtractText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", ErrNotDocx
	}
	var f *zip.File
	for _, zf := range zr.File {
		if zf.Name == "word/document.xml" {
			f = zf
			break
		}
	}
	if f == nil {
		return "", ErrNotDocx
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	lr := &io.LimitedReader{R: rc, N: MaxUncompressed + 1}
	dec := xml.NewDecoder(lr)
	var b strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if lr.N <= 0 {
				return "", errors.New("document too large after decompression")
			}
			return "", ErrNotDocx
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteByte('\t')
			case "br", "cr":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n\n")
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}
