package upload

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUploadPrepared_MeasureWriteAndCancel(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "example.pdf")
	require.NoError(t, err)
	data := []byte("%PDF-1.4\nexample\n%%EOF\n")
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1024))
	defer request.MultipartForm.RemoveAll()
	file := request.MultipartForm.File["files"][0]
	opts := Options{Dir: filepath.Join(t.TempDir(), "application_files"), MaxFileSize: 1024, AllowedTypes: []string{"application/pdf"}}
	size, err := MeasureFile(context.Background(), file, 1024)
	require.NoError(t, err)
	require.EqualValues(t, len(data), size)
	prepared, err := PrepareFile(context.Background(), file, opts, size)
	require.NoError(t, err)
	_, err = os.Stat(opts.Dir)
	require.True(t, os.IsNotExist(err), "preparation must not create permanent storage")
	_, err = PrepareFile(context.Background(), file, opts, size+1)
	require.Error(t, err)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = PrepareFile(canceled, file, opts, size)
	require.ErrorIs(t, err, context.Canceled)
	_, err = MeasureFile(context.Background(), file, 4)
	require.Error(t, err)
	saved, err := WritePrepared(context.Background(), prepared, opts, "example.pdf")
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), saved.Size)
	actual, err := os.ReadFile(filepath.Join(opts.Dir, "example.pdf"))
	require.NoError(t, err)
	require.Equal(t, data, actual)
	_, err = WritePrepared(context.Background(), prepared, opts, "example.pdf")
	require.Error(t, err, "must not overwrite existing storage")
	_, err = WritePrepared(context.Background(), prepared, opts, "../other.pdf")
	require.Error(t, err)
}
