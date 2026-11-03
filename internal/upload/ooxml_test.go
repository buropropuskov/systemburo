package upload

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf16"

	"systemburo/internal/apperr"

	"github.com/stretchr/testify/require"
)

type officeTestPart struct {
	name string
	data []byte
}

func officeAttribute(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}

// Минимальные независимые OPC-образцы: главная часть задаётся связью, не путём.
func officeTestParts(extension, variant, name, target string) ([]officeTestPart, string) {
	var mime, contentType, root, ns string
	switch extension {
	case ".docx":
		mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		contentType = mime + ".main+xml"
		root, ns = "document", "wordprocessingml"
	case ".xlsx":
		mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
		root, ns = "workbook", "spreadsheetml"
	case ".pptx":
		mime = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		contentType = mime + ".main+xml"
		root, ns = "presentation", "presentationml"
	default:
		panic("unsupported fixture")
	}
	if target == "" {
		target = name
	}
	rel := "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	namespace := "http://schemas.openxmlformats.org/" + ns + "/2006/main"
	if variant == "strict" {
		rel = "http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument"
		namespace = "http://purl.oclc.org/ooxml/" + ns + "/main"
	}
	return []officeTestPart{
		{"[Content_Types].xml", []byte(fmt.Sprintf(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/%s" ContentType="%s"/></Types>`, officeAttribute(name), contentType))},
		{"_rels/.rels", []byte(fmt.Sprintf(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="main" Type="%s" Target="%s"/><Relationship Id="link" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.invalid/never-fetch" TargetMode="External"/></Relationships>`, rel, officeAttribute(target)))},
		{name, []byte(fmt.Sprintf(`<m:%s xmlns:m="%s"/>`, root, namespace))},
	}, mime
}

func officeTestZIP(t *testing.T, parts []officeTestPart) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, part := range parts {
		entry, err := w.CreateHeader(&zip.FileHeader{Name: part.name, Method: zip.Store})
		require.NoError(t, err)
		_, err = entry.Write(part.data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return out.Bytes()
}

func validateOfficeBytes(ctx context.Context, data []byte, extension string) (string, error) {
	return ValidateOOXML(ctx, bytes.NewReader(data), int64(len(data)), extension)
}

func requireOfficeValidation(t *testing.T, err error) {
	t.Helper()
	var domain *apperr.Error
	require.ErrorAs(t, err, &domain)
	require.Equal(t, http.StatusBadRequest, domain.Code)
	// В публичный текст не попадают имена частей или детали ZIP/XML parser.
	require.Contains(t, []string{"Недопустимый офисный документ", "Распакованный офисный документ превышает допустимый размер"}, domain.Message)
}

func TestValidateOOXML_FormatsNamespacesAndPartURIs(t *testing.T) {
	for _, extension := range []string{".docx", ".xlsx", ".pptx"} {
		for _, variant := range []string{"transitional", "strict"} {
			t.Run(extension+"/"+variant, func(t *testing.T) {
				parts, mime := officeTestParts(extension, variant, "custom/main.xml", "/custom/main.xml")
				// ZIP-порядок не обязан начинаться с metadata.
				parts[0], parts[2] = parts[2], parts[0]
				got, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), extension)
				require.NoError(t, err)
				require.Equal(t, mime, got)
			})
		}
	}
	for _, tc := range []struct{ name, target string }{
		{"資料/Глава 1.xml", "./%E8%B3%87%E6%96%99/%D0%93%D0%BB%D0%B0%D0%B2%D0%B0%201.xml"},
		{"custom/a%20b.xml", "custom/a%20b.xml"},
		{"custom/Part.xml", "custom/PART.XML"},
		{"custom/main.xml", "custom/sub/../main.xml"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			parts, want := officeTestParts(".docx", "transitional", tc.name, tc.target)
			got, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), ".docx")
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

func officeUTF16(data []byte, bigEndian bool) []byte {
	return officeUTF16Declared(data, bigEndian, "UTF-16", true)
}

func officeUTF16Declared(data []byte, bigEndian bool, label string, bom bool) []byte {
	order := binary.ByteOrder(binary.LittleEndian)
	result := []byte{0xff, 0xfe}
	if bigEndian {
		order, result = binary.BigEndian, []byte{0xfe, 0xff}
	}
	if !bom {
		result = nil
	}
	text := `<?xml version="1.0" encoding="` + label + `"?>` + string(data)
	for _, unit := range utf16.Encode([]rune(text)) {
		var pair [2]byte
		order.PutUint16(pair[:], unit)
		result = append(result, pair[:]...)
	}
	return result
}

func TestValidateOOXML_UTF16ExplicitEndianWithoutBOM(t *testing.T) {
	for _, extension := range []string{".docx", ".xlsx", ".pptx"} {
		for _, tc := range []struct {
			label     string
			bigEndian bool
		}{{"utf-16LE", false}, {"UTF-16be", true}} {
			t.Run(extension+"/"+tc.label, func(t *testing.T) {
				parts, want := officeTestParts(extension, "transitional", "任意/главная.xml", "任意/главная.xml")
				for i := range parts {
					parts[i].data = officeUTF16Declared(parts[i].data, tc.bigEndian, tc.label, false)
				}
				got, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), extension)
				require.NoError(t, err)
				require.Equal(t, want, got)
			})
		}
	}
}

func TestValidateOOXML_UTF16DeclaredEncodingMismatch(t *testing.T) {
	for _, tc := range []struct {
		label     string
		bigEndian bool
	}{{"UTF-16BE", false}, {"UTF-16LE", true}, {"UTF-8", false}, {"UTF-8", true}} {
		for _, bom := range []bool{false, true} {
			for part := 0; part < 3; part++ {
				t.Run(fmt.Sprintf("%s/big=%t/bom=%t/part=%d", tc.label, tc.bigEndian, bom, part), func(t *testing.T) {
					parts, _ := officeTestParts(".docx", "strict", "custom/main.xml", "")
					parts[part].data = officeUTF16Declared(parts[part].data, tc.bigEndian, tc.label, bom)
					_, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), ".docx")
					requireOfficeValidation(t, err)
				})
			}
		}
	}
}

func TestValidateOOXML_UTF8BOMAndUTF16(t *testing.T) {
	for _, extension := range []string{".docx", ".xlsx", ".pptx"} {
		for _, mode := range []string{"utf8-bom", "utf16le", "utf16be"} {
			t.Run(extension+"/"+mode, func(t *testing.T) {
				parts, want := officeTestParts(extension, "strict", "任意/главная.xml", "任意/главная.xml")
				for i := range parts {
					if mode == "utf8-bom" {
						parts[i].data = append([]byte{0xef, 0xbb, 0xbf}, parts[i].data...)
					} else {
						parts[i].data = officeUTF16(parts[i].data, mode == "utf16be")
					}
				}
				got, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), extension)
				require.NoError(t, err)
				require.Equal(t, want, got)
			})
		}
	}
}

func TestValidateOOXML_RejectsInvalidPackages(t *testing.T) {
	cases := []struct {
		name   string
		change func([]officeTestPart) []officeTestPart
	}{
		{"missing_types", func(p []officeTestPart) []officeTestPart { return p[1:] }},
		{"missing_relationships", func(p []officeTestPart) []officeTestPart { return append(p[:1], p[2:]...) }},
		{"missing_main", func(p []officeTestPart) []officeTestPart { return p[:2] }},
		{"duplicate_name", func(p []officeTestPart) []officeTestPart { return append(p, p[2]) }},
		{"case_ambiguous_name", func(p []officeTestPart) []officeTestPart {
			return append(p, officeTestPart{"CUSTOM/MAIN.XML", p[2].data})
		}},
		{"escaped_ambiguous_name", func(p []officeTestPart) []officeTestPart {
			return append(p, officeTestPart{"custom/%6dain.xml", p[2].data})
		}},
		{"traversal", func(p []officeTestPart) []officeTestPart { return append(p, officeTestPart{"../outside", nil}) }},
		{"absolute_zip_path", func(p []officeTestPart) []officeTestPart { return append(p, officeTestPart{"/absolute", nil}) }},
		{"backslash", func(p []officeTestPart) []officeTestPart { return append(p, officeTestPart{`custom\outside`, nil}) }},
		{"encoded_separator", func(p []officeTestPart) []officeTestPart { return append(p, officeTestPart{"custom%2Fother.xml", nil}) }},
		{"malformed_types", func(p []officeTestPart) []officeTestPart { p[0].data = []byte("<Types>"); return p }},
		{"malformed_rels", func(p []officeTestPart) []officeTestPart { p[1].data = []byte("not XML"); return p }},
		{"malformed_main", func(p []officeTestPart) []officeTestPart { p[2].data = []byte("<broken>"); return p }},
		{"main_root_mismatch", func(p []officeTestPart) []officeTestPart {
			p[2].data = []byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`)
			return p
		}},
		{"two_roots", func(p []officeTestPart) []officeTestPart { p[2].data = append(p[2].data, p[2].data...); return p }},
		{"doctype_external_entity", func(p []officeTestPart) []officeTestPart {
			p[2].data = append([]byte(`<!DOCTYPE m:document SYSTEM "https://example.invalid/no-access">`), p[2].data...)
			return p
		}},
		{"external_main", func(p []officeTestPart) []officeTestPart {
			p[1].data = bytes.Replace(p[1].data, []byte(`Id="main"`), []byte(`Id="main" TargetMode="External"`), 1)
			return p
		}},
		{"remote_main_without_mode", func(p []officeTestPart) []officeTestPart {
			p[1].data = bytes.Replace(p[1].data, []byte(`Target="custom/main.xml"`), []byte(`Target="https://example.invalid/main.xml"`), 1)
			return p
		}},
		{"duplicate_main_relationship", func(p []officeTestPart) []officeTestPart {
			p[1].data = bytes.Replace(p[1].data, []byte(`</Relationships>`), []byte(`<Relationship Id="other" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="custom/main.xml"/></Relationships>`), 1)
			return p
		}},
		{"duplicate_attribute", func(p []officeTestPart) []officeTestPart {
			p[1].data = bytes.Replace(p[1].data, []byte(`Id="main"`), []byte(`Id="main" Id="other"`), 1)
			return p
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts, _ := officeTestParts(".docx", "transitional", "custom/main.xml", "")
			_, err := validateOfficeBytes(context.Background(), officeTestZIP(t, tc.change(parts)), ".docx")
			requireOfficeValidation(t, err)
		})
	}
	parts, _ := officeTestParts(".xlsx", "transitional", "xl/workbook.xml", "")
	_, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), ".docx")
	requireOfficeValidation(t, err)
	for _, data := range [][]byte{[]byte("PK\x03\x04garbage"), officeTestZIP(t, []officeTestPart{{"readme.txt", []byte("arbitrary zip")}})} {
		_, err := validateOfficeBytes(context.Background(), data, ".docx")
		requireOfficeValidation(t, err)
	}
}

func TestValidateOOXML_AllEntriesCRCAndTruncation(t *testing.T) {
	parts, _ := officeTestParts(".docx", "transitional", "custom/main.xml", "")
	payload := []byte("unused-binary-must-also-be-checked")
	parts = append(parts, officeTestPart{"media/data.bin", payload})
	data := officeTestZIP(t, parts)
	_, err := validateOfficeBytes(context.Background(), data[:len(data)-22], ".docx")
	requireOfficeValidation(t, err)
	offset := bytes.Index(data, payload)
	require.GreaterOrEqual(t, offset, 0)
	data[offset] ^= 0x01
	_, err = validateOfficeBytes(context.Background(), data, ".docx")
	requireOfficeValidation(t, err)
}

func TestValidateOOXML_ActualBudgetExactBoundary(t *testing.T) {
	parts, _ := officeTestParts(".docx", "transitional", "custom/main.xml", "")
	parts = append(parts, officeTestPart{"media/data.bin", bytes.Repeat([]byte{0}, 1234)})
	var total int64
	for _, part := range parts {
		total += int64(len(part.data))
	}
	data := officeTestZIP(t, parts)
	_, err := validateOOXMLBudget(context.Background(), bytes.NewReader(data), int64(len(data)), ".docx", total)
	require.NoError(t, err, "metadata and main part must be counted once, not twice")
	_, err = validateOOXMLBudget(context.Background(), bytes.NewReader(data), int64(len(data)), ".docx", total-1)
	requireOfficeValidation(t, err)
	require.Contains(t, err.Error(), "превышает")
}

type officeZeros struct{}

func (officeZeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestValidateOOXML_PublicBudgetHighlyCompressedPart(t *testing.T) {
	parts, _ := officeTestParts(".xlsx", "transitional", "xl/workbook.xml", "")
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, part := range parts {
		entry, err := w.Create(part.name)
		require.NoError(t, err)
		_, err = entry.Write(part.data)
		require.NoError(t, err)
	}
	entry, err := w.Create("media/zeros.bin")
	require.NoError(t, err)
	_, err = io.CopyN(entry, officeZeros{}, 64<<20)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.Less(t, out.Len(), 1<<20, "compressed input is small; decompressed contents exceed the whole-package budget")
	_, err = validateOfficeBytes(context.Background(), out.Bytes(), ".xlsx")
	requireOfficeValidation(t, err)
	require.Contains(t, err.Error(), "превышает")
}

func TestValidateOOXML_ForgedZIP64DeclaredSize(t *testing.T) {
	parts, _ := officeTestParts(".docx", "transitional", "custom/main.xml", "")
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, part := range parts {
		entry, err := w.Create(part.name)
		require.NoError(t, err)
		_, err = entry.Write(part.data)
		require.NoError(t, err)
	}
	payload := []byte("actual-small-bytes")
	entry, err := w.CreateRaw(&zip.FileHeader{Name: "media/forged.bin", Method: zip.Store,
		UncompressedSize64: ^uint64(0), CompressedSize64: uint64(len(payload)), CRC32: crc32.ChecksumIEEE(payload)})
	require.NoError(t, err)
	_, err = entry.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NotPanics(t, func() {
		_, err = validateOfficeBytes(context.Background(), out.Bytes(), ".docx")
		requireOfficeValidation(t, err)
	})
}

type officeCancelReaderAt struct {
	src    *bytes.Reader
	offset int64
	cancel context.CancelFunc
}

func (r officeCancelReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	n, err := r.src.ReadAt(p, offset)
	if offset == r.offset {
		r.cancel()
	}
	return n, err
}

func TestValidateOOXML_ContextCancellation(t *testing.T) {
	parts, _ := officeTestParts(".docx", "transitional", "custom/main.xml", "")
	parts = append(parts, officeTestPart{"media/large.bin", bytes.Repeat([]byte{0x5a}, 128<<10)})
	data := officeTestZIP(t, parts)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := validateOfficeBytes(ctx, data, ".docx")
	require.ErrorIs(t, err, context.Canceled)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	offset, err := zr.File[len(zr.File)-1].DataOffset()
	require.NoError(t, err)
	_, err = ValidateOOXML(ctx, officeCancelReaderAt{bytes.NewReader(data), offset, cancel}, int64(len(data)), ".docx")
	require.ErrorIs(t, err, context.Canceled)
	var domain *apperr.Error
	require.False(t, errors.As(err, &domain), "cancellation is not a malformed-file error")
}

func TestValidateOOXML_DefaultContentTypeAndInvalidInputs(t *testing.T) {
	parts, mime := officeTestParts(".docx", "transitional", "custom/main.docpart", "")
	parts[0].data = []byte(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="docpart" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	got, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), ".docx")
	require.NoError(t, err)
	require.Equal(t, mime, got)
	for _, extension := range []string{"", ".zip", ".docm"} {
		_, err := validateOfficeBytes(context.Background(), officeTestZIP(t, parts), extension)
		requireOfficeValidation(t, err)
	}
	_, err = ValidateOOXML(context.Background(), nil, 1, ".docx")
	requireOfficeValidation(t, err)
	_, err = ValidateOOXML(context.Background(), strings.NewReader(""), 0, ".docx")
	requireOfficeValidation(t, err)
}
