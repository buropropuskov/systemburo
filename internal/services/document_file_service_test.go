package services

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
)

// makeFileHeader создаёт тестовый multipart.FileHeader с указанным именем и содержимым.
func makeFileHeader(t *testing.T, name string, content []byte) *multipart.FileHeader {
	t.Helper()

	// Записываем содержимое во временный файл чтобы multipart.FileHeader умел его открыть
	tmp, err := os.CreateTemp(t.TempDir(), "testfile-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if _, err := tmp.Write(content); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	// Эмулируем FileHeader через обёртку
	return &multipart.FileHeader{
		Filename: name,
		Size:     int64(len(content)),
		Header:   textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}},
		// Переопределяем Open через workaround: используем специально созданный файл
	}
}

// testableFileHeader создаёт *multipart.FileHeader, Open которого возвращает content.
// multipart.FileHeader хранит путь приватно, поэтому используем tmpdir и патчим Open.
func testableFile(t *testing.T, dir, name string, content []byte) *multipart.FileHeader {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Создаём FileHeader с нужными данными через multipart-запись
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form: %v", err)
	}
	w.Close()

	r := multipart.NewReader(&buf, w.Boundary())
	form, err := r.ReadForm(int64(len(content)) + 1024)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	files := form.File["file"]
	if len(files) == 0 {
		t.Fatal("no files in form")
	}
	return files[0]
}

func TestDocumentFileService_ValidateMagicBytes(t *testing.T) {
	tests := []struct {
		name    string
		fname   string
		content []byte
		wantErr bool
	}{
		{
			name:    "pdf_valid",
			fname:   "doc.pdf",
			content: append([]byte{0x25, 0x50, 0x44, 0x46}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: false,
		},
		{
			name:    "docx_valid",
			fname:   "doc.docx",
			content: append([]byte{0x50, 0x4B, 0x03, 0x04}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: false,
		},
		{
			name:    "xlsx_valid",
			fname:   "sheet.xlsx",
			content: append([]byte{0x50, 0x4B, 0x03, 0x04}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: false,
		},
		{
			name:    "pptx_valid",
			fname:   "pres.pptx",
			content: append([]byte{0x50, 0x4B, 0x03, 0x04}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: false,
		},
		{
			name:    "doc_valid_ole2",
			fname:   "legacy.doc",
			content: append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: false,
		},
		{
			name:    "pdf_wrong_ext_docx",
			fname:   "renamed.docx",
			content: append([]byte{0x25, 0x50, 0x44, 0x46}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: true,
		},
		{
			name:    "exe_renamed_as_pdf",
			fname:   "virus.pdf",
			content: append([]byte{0x4D, 0x5A, 0x00, 0x00}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: true,
		},
		{
			name:    "forbidden_extension",
			fname:   "script.js",
			content: []byte("alert('xss')"),
			wantErr: true,
		},
		{
			name:    "doc_with_ooxml_signature",
			fname:   "fake.doc",
			content: append([]byte{0x50, 0x4B, 0x03, 0x04}, bytes.Repeat([]byte("x"), 100)...),
			wantErr: true, // .doc должен иметь OLE2, а не OOXML
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			svc := NewDocumentFileService(dir)
			// UploadDir = dir/documents
			uploadDir := svc.UploadDir()

			fh := testableFile(t, t.TempDir(), tc.fname, tc.content)

			_, _, err := svc.Save(context.Background(), fh, 10*1024*1024)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else {
					// Проверяем что файл создан в upload dir
					entries, _ := os.ReadDir(uploadDir)
					if len(entries) == 0 {
						t.Errorf("expected file in upload dir %s, none found", uploadDir)
					}
				}
			}
		})
	}
}

func TestDocumentFileService_SizeLimit(t *testing.T) {
	dir := t.TempDir()
	svc := NewDocumentFileService(dir)

	// 5 байт валидного PDF-заголовка + padding, но заявляем size = 100 MB -> отказ по размеру
	content := append([]byte{0x25, 0x50, 0x44, 0x46}, bytes.Repeat([]byte("x"), 100)...)
	fh := testableFile(t, t.TempDir(), "big.pdf", content)
	fh.Size = 200 * 1024 * 1024 // 200 MB

	_, _, err := svc.Save(context.Background(), fh, 10*1024*1024) // 10 MB limit
	if err == nil {
		t.Error("expected size error, got nil")
	}
}

func TestMatchesMagic(t *testing.T) {
	tests := []struct {
		name   string
		header []byte
		family magicFamily
		want   bool
	}{
		{"pdf ok", []byte{0x25, 0x50, 0x44, 0x46, 0x00}, familyPDF, true},
		{"pdf bad", []byte{0x00, 0x50, 0x44, 0x46, 0x00}, familyPDF, false},
		{"ooxml ok", []byte{0x50, 0x4B, 0x03, 0x04, 0x00}, familyOOXML, true},
		{"ooxml bad", []byte{0x50, 0x4B, 0x00, 0x04, 0x00}, familyOOXML, false},
		{"ole2 ok", []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, familyOLE2, true},
		{"ole2 short", []byte{0xD0, 0xCF, 0x11, 0xE0}, familyOLE2, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesMagic(tc.header, tc.family)
			if got != tc.want {
				t.Errorf("matchesMagic(%x, %d) = %v, want %v", tc.header, tc.family, got, tc.want)
			}
		})
	}
}

// Проверяем что Delete не паникует на пустом storedName и несуществующем файле.
func TestDocumentFileService_Delete(t *testing.T) {
	dir := t.TempDir()
	svc := NewDocumentFileService(dir)

	svc.Delete("")                     // не паникует
	svc.Delete("nonexistent-uuid.pdf") // не паникует
}

// Smoke-test: стрим реального файла через c.File() использует UploadDir.
func TestDocumentFileService_UploadDir(t *testing.T) {
	dir := t.TempDir()
	svc := NewDocumentFileService(dir)
	if svc.UploadDir() == "" {
		t.Error("UploadDir should not be empty")
	}
	_ = io.Discard
}
