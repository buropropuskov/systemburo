package upload

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

const (
	ooxmlIntegrationWord   = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	ooxmlIntegrationSheet  = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	ooxmlIntegrationSlides = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

type ooxmlIntegrationPart struct{ name, body string }
type ooxmlIntegrationFile struct {
	name string
	data []byte
}

func ooxmlIntegrationZIP(t *testing.T, entries ...ooxmlIntegrationPart) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for _, part := range entries {
		entry, err := writer.Create(part.name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, part.body)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return data.Bytes()
}

func ooxmlIntegrationOPC(t *testing.T, extension string) []byte {
	t.Helper()
	var main, contentType, xml string
	switch extension {
	case ".docx":
		main, contentType = "word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
		xml = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/></w:body></w:document>`
	case ".xlsx":
		main, contentType = "xl/workbook.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
		xml = `<s:workbook xmlns:s="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><s:sheets/></s:workbook>`
	case ".pptx":
		main, contentType = "ppt/presentation.xml", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
		xml = `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`
	default:
		t.Fatalf("unsupported fixture extension %q", extension)
	}
	return ooxmlIntegrationZIP(t,
		ooxmlIntegrationPart{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/` + main + `" ContentType="` + contentType + `"/></Types>`},
		ooxmlIntegrationPart{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="` + main + `"/></Relationships>`},
		ooxmlIntegrationPart{main, xml})
}

func ooxmlIntegrationForm(t *testing.T, files ...ooxmlIntegrationFile) echo.Context {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.name)
		require.NoError(t, err)
		_, err = part.Write(file.data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	t.Cleanup(func() { require.NoError(t, request.MultipartForm.RemoveAll()) })
	return echo.New().NewContext(request, httptest.NewRecorder())
}

func TestOOXMLIntegration_PrepareAndSaveChooseActualOfficeMIME(t *testing.T) {
	for _, tc := range []struct{ extension, mime string }{{".docx", ooxmlIntegrationWord}, {".xlsx", ooxmlIntegrationSheet}, {".pptx", ooxmlIntegrationSlides}} {
		t.Run(tc.extension, func(t *testing.T) {
			content := ooxmlIntegrationOPC(t, tc.extension)
			c := ooxmlIntegrationForm(t, ooxmlIntegrationFile{"fixture" + tc.extension, content})
			file := c.Request().MultipartForm.File["files"][0]
			dir := filepath.Join(t.TempDir(), "permanent")
			// The real MIME is never the first allowed type; a generic ZIP must
			// not be promoted to whichever Office family the allow-list lists first.
			allowed := []string{"application/pdf", ooxmlIntegrationSlides, ooxmlIntegrationSheet, ooxmlIntegrationWord}
			if tc.mime == ooxmlIntegrationSlides {
				allowed = []string{"application/pdf", ooxmlIntegrationWord, ooxmlIntegrationSheet, ooxmlIntegrationSlides}
			}
			opts := Options{Dir: dir, MaxFileSize: 10 * 1024 * 1024, AllowedTypes: allowed}
			prepared, err := PrepareFile(context.Background(), file, opts, int64(len(content)))
			require.NoError(t, err)
			require.Equal(t, tc.mime, prepared.Metadata.DetectedMime)
			require.Equal(t, content, prepared.Content)
			_, err = os.Stat(dir)
			require.True(t, os.IsNotExist(err), "preparation must not create permanent storage")
			saved, err := SaveMultipart(c, "files", opts)
			require.NoError(t, err)
			require.Len(t, saved, 1)
			require.Equal(t, tc.mime, saved[0].DetectedMime)
			require.Equal(t, tc.extension, filepath.Ext(saved[0].StoredName))
			stored, err := os.ReadFile(filepath.Join(dir, saved[0].StoredName))
			require.NoError(t, err)
			require.Equal(t, content, stored)
		})
	}
}

func TestOOXMLIntegration_RejectsNonOfficeAndRenamedOfficeWithoutFiles(t *testing.T) {
	generic := ooxmlIntegrationZIP(t, ooxmlIntegrationPart{"readme.txt", "not an Office package"})
	for _, tc := range []struct {
		name    string
		content []byte
	}{
		{"generic.docx", generic}, {"renamed.xlsx", ooxmlIntegrationOPC(t, ".docx")},
		{"padding.pptx", append([]byte{'P', 'K', 3, 4}, bytes.Repeat([]byte{'x'}, 100)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := ooxmlIntegrationForm(t, ooxmlIntegrationFile{tc.name, tc.content})
			dir := filepath.Join(t.TempDir(), "permanent")
			opts := Options{Dir: dir, MaxFileSize: 10 * 1024 * 1024, AllowedTypes: []string{ooxmlIntegrationWord, ooxmlIntegrationSheet, ooxmlIntegrationSlides}}
			_, err := PrepareFile(context.Background(), c.Request().MultipartForm.File["files"][0], opts, int64(len(tc.content)))
			require.Error(t, err)
			_, err = os.Stat(dir)
			require.True(t, os.IsNotExist(err), "rejection during preparation must precede permanent storage")
			_, err = SaveMultipart(c, "files", opts)
			require.Error(t, err)
			entries, err := os.ReadDir(dir)
			if err != nil {
				require.True(t, os.IsNotExist(err), "unexpected storage error: %v", err)
			}
			require.Empty(t, entries)
		})
	}
}

func TestOOXMLIntegration_MixedBatchCleansOnlyItsOwnFiles(t *testing.T) {
	dir := t.TempDir()
	existing := []byte("existing document must survive failed upload")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.docx"), existing, 0600))
	c := ooxmlIntegrationForm(t,
		ooxmlIntegrationFile{"valid.docx", ooxmlIntegrationOPC(t, ".docx")},
		ooxmlIntegrationFile{"valid.xlsx", ooxmlIntegrationOPC(t, ".xlsx")},
		ooxmlIntegrationFile{"renamed.pptx", ooxmlIntegrationOPC(t, ".docx")})
	saved, err := SaveMultipart(c, "files", Options{Dir: dir, MaxFileSize: 10 * 1024 * 1024, AllowedTypes: []string{ooxmlIntegrationWord, ooxmlIntegrationSheet, ooxmlIntegrationSlides}})
	require.Error(t, err)
	require.Empty(t, saved)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "existing.docx", entries[0].Name())
	actual, err := os.ReadFile(filepath.Join(dir, "existing.docx"))
	require.NoError(t, err)
	require.Equal(t, existing, actual)
}

func TestOOXMLIntegration_AllowListAndActualSizeRemainAuthoritative(t *testing.T) {
	for _, scenario := range []string{"xlsx_not_allowed_by_word_only", "docx_actual_size"} {
		t.Run(scenario, func(t *testing.T) {
			extension := ".xlsx"
			if scenario == "docx_actual_size" {
				extension = ".docx"
			}
			content := ooxmlIntegrationOPC(t, extension)
			c := ooxmlIntegrationForm(t, ooxmlIntegrationFile{"fixture" + extension, content})
			file := c.Request().MultipartForm.File["files"][0]
			dir := filepath.Join(t.TempDir(), "permanent")
			opts := Options{Dir: dir, MaxFileSize: int64(len(content)), AllowedTypes: []string{ooxmlIntegrationWord}}
			if scenario == "docx_actual_size" {
				file.Size = 1
				opts.MaxFileSize--
			}
			// Use the measured size, not the deliberately falsified FileHeader.
			_, err := PrepareFile(context.Background(), file, opts, int64(len(content)))
			require.Error(t, err)
			_, err = os.Stat(dir)
			require.True(t, os.IsNotExist(err))
			saved, err := SaveMultipart(c, "files", opts)
			require.Error(t, err)
			require.Empty(t, saved)
			entries, err := os.ReadDir(dir)
			if err != nil {
				require.True(t, os.IsNotExist(err), "unexpected storage error: %v", err)
			}
			require.Empty(t, entries)
		})
	}
}
