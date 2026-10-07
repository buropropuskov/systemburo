package upload

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type testCloser struct{ close func() error }

func (c testCloser) Close() error { return c.close() }

type testWriter struct {
	io.Writer
	close func() error
}

func (w testWriter) Close() error { return w.close() }

func TestPipelineFinalizationFailures(t *testing.T) {
	for _, failure := range []string{"write", "writer-close", "file-close", "cleanup", "success"} {
		t.Run(failure, func(t *testing.T) {
			var order []string
			boom := errors.New("synthetic failure")
			var dst bytes.Buffer
			writer := testWriter{Writer: &dst, close: func() error {
				order = append(order, "writer")
				if failure == "writer-close" || failure == "cleanup" {
					return boom
				}
				return nil
			}}
			file := testCloser{close: func() error {
				order = append(order, "file")
				if failure == "file-close" {
					return boom
				}
				return nil
			}}
			var src io.Reader = bytes.NewBufferString("content")
			if failure == "write" {
				src = failingReader{}
			}
			_, err := finishWrite(writer, file, src, 100, func() error {
				order = append(order, "cleanup")
				if failure == "cleanup" {
					return boom
				}
				return nil
			})
			if failure == "success" {
				require.NoError(t, err)
				require.Equal(t, []string{"writer", "file"}, order)
			} else {
				require.Error(t, err)
				require.Equal(t, []string{"writer", "file", "cleanup"}, order)
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("synthetic read failure") }

func TestPipelineBatchRollback(t *testing.T) {
	for _, bad := range []int{1, 2} {
		t.Run(string(rune('0'+bad)), func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.pdf"), []byte("existing"), 0600))
			var body bytes.Buffer
			w := multipart.NewWriter(&body)
			for i := 0; i < 3; i++ {
				part, err := w.CreateFormFile("files", "file.pdf")
				require.NoError(t, err)
				data := []byte("%PDF-1.4 synthetic")
				if i == bad {
					data = []byte("invalid")
				}
				_, err = part.Write(data)
				require.NoError(t, err)
			}
			require.NoError(t, w.Close())
			r := httptest.NewRequest("POST", "/", &body)
			r.Header.Set("Content-Type", w.FormDataContentType())
			c := echo.New().NewContext(r, httptest.NewRecorder())
			_, err := SaveMultipart(c, "files", Options{Dir: dir, MaxFileSize: 100, AllowedTypes: []string{"application/pdf"}})
			require.Error(t, err)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			require.Equal(t, "existing.pdf", entries[0].Name())
		})
	}
}

func TestPipelineActualSizeLimit(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	p, err := w.CreateFormFile("files", "test.pdf")
	require.NoError(t, err)
	_, err = p.Write([]byte("%PDF-1.4 abcdefghijklmnop"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	r := multipart.NewReader(&body, w.Boundary())
	form, err := r.ReadForm(1000)
	require.NoError(t, err)
	defer form.RemoveAll()
	form.File["files"][0].Size = 1
	dir := t.TempDir()
	_, err = saveOne(form.File["files"][0], Options{Dir: dir, MaxFileSize: 10, AllowedTypes: []string{"application/pdf"}})
	require.Error(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}
