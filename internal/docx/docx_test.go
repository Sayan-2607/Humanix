package docx

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeDocx(t *testing.T, body string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("word/document.xml")
	w.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body + `</w:body></w:document>`))
	zw.Close()
	return buf.Bytes()
}

func TestExtract(t *testing.T) {
	d := makeDocx(t, `<w:p><w:r><w:t>Hello</w:t></w:r><w:r><w:t xml:space="preserve"> world &amp; co.</w:t></w:r></w:p><w:p><w:r><w:t>Second</w:t><w:tab/><w:t>para</w:t></w:r></w:p>`)
	got, err := ExtractText(d)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello world & co.\n\nSecond\tpara" {
		t.Fatalf("%q", got)
	}
	if _, err := ExtractText([]byte("not a zip")); err != ErrNotDocx {
		t.Fatal("expected ErrNotDocx")
	}
}
