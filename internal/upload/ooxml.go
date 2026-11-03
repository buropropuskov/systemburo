package upload

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"systemburo/internal/apperr"

	"golang.org/x/text/encoding/unicode"
)

// MaxOOXMLUncompressedBytes — общий предел фактически прочитанных распакованных байтов.
const MaxOOXMLUncompressedBytes int64 = 64 << 20

const (
	ooxmlTypesNS       = "http://schemas.openxmlformats.org/package/2006/content-types"
	ooxmlRelsNS        = "http://schemas.openxmlformats.org/package/2006/relationships"
	ooxmlMainRel       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	ooxmlStrictMainRel = "http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument"
)

var (
	errOOXMLInvalid = errors.New("invalid OOXML package")
	errOOXMLBudget  = errors.New("OOXML uncompressed budget exceeded")
)

type ooxmlFormat struct {
	mime, contentType, root, namespace, strictNamespace string
}

func ooxmlExpectedFormat(extension string) (ooxmlFormat, bool) {
	switch extension {
	case ".docx":
		return ooxmlFormat{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml", "document", "http://schemas.openxmlformats.org/wordprocessingml/2006/main", "http://purl.oclc.org/ooxml/wordprocessingml/main"}, true
	case ".xlsx":
		return ooxmlFormat{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml", "workbook", "http://schemas.openxmlformats.org/spreadsheetml/2006/main", "http://purl.oclc.org/ooxml/spreadsheetml/main"}, true
	case ".pptx":
		return ooxmlFormat{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml", "presentation", "http://schemas.openxmlformats.org/presentationml/2006/main", "http://purl.oclc.org/ooxml/presentationml/main"}, true
	}
	return ooxmlFormat{}, false
}

// ValidateOOXML подтверждает OPC-структуру, тип главной XML-части и читает все части
// до EOF/проверки CRC. Это не антивирус и не полная XSD-валидация. Размер сжатого
// источника проверяет вызывающий: zip.NewReader читает центральный каталог до
// наших проверок и сам по себе не ограничивается распакованным бюджетом.
func ValidateOOXML(ctx context.Context, src io.ReaderAt, actualSize int64, extension string) (string, error) {
	return validateOOXMLBudget(ctx, src, actualSize, extension, MaxOOXMLUncompressedBytes)
}

func validateOOXMLBudget(ctx context.Context, src io.ReaderAt, actualSize int64, extension string, limit int64) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	format, supported := ooxmlExpectedFormat(extension)
	if !supported || src == nil || actualSize <= 0 || limit < 0 {
		return "", apperr.Validation("Недопустимый офисный документ")
	}
	err := inspectOOXML(ctx, src, actualSize, format, limit)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if errors.Is(err, errOOXMLBudget) {
		return "", apperr.Validation("Распакованный офисный документ превышает допустимый размер")
	}
	if err != nil {
		return "", apperr.Validation("Недопустимый офисный документ")
	}
	return format.mime, nil
}

func inspectOOXML(ctx context.Context, src io.ReaderAt, size int64, format ooxmlFormat, limit int64) error {
	archive, err := zip.NewReader(src, size)
	if err != nil {
		return err
	}
	parts := make(map[string]*zip.File, len(archive.File))
	seen := make(map[string]bool, len(archive.File))
	for _, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		directory := strings.HasSuffix(file.Name, "/")
		name := file.Name
		if directory {
			name = strings.TrimSuffix(name, "/")
		}
		key, err := ooxmlPartName(name, false)
		if err != nil {
			return err
		}
		seenKey := key
		if directory {
			seenKey += "/"
		}
		if seen[seenKey] {
			return errOOXMLInvalid
		}
		seen[seenKey] = true
		if !directory {
			parts[key] = file
		}
	}
	typesFile, relsFile := parts["[content_types].xml"], parts["_rels/.rels"]
	if typesFile == nil || relsFile == nil {
		return errOOXMLInvalid
	}
	remaining := limit
	overrides, defaults := map[string]string{}, map[string]string{}
	err = readOOXMLPart(ctx, typesFile, &remaining, func(r io.Reader) error {
		return walkOOXML(r, func(start xml.StartElement, depth int) error {
			if depth == 1 {
				if start.Name != (xml.Name{Space: ooxmlTypesNS, Local: "Types"}) {
					return errOOXMLInvalid
				}
			} else if depth == 2 && start.Name.Space == ooxmlTypesNS {
				contentType, err := ooxmlAttribute(start, "ContentType")
				if err != nil {
					return err
				}
				switch start.Name.Local {
				case "Override":
					name, err := ooxmlAttribute(start, "PartName")
					if err != nil || !strings.HasPrefix(name, "/") {
						return errOOXMLInvalid
					}
					key, err := ooxmlPartName(name, true)
					if err != nil || overrides[key] != "" {
						return errOOXMLInvalid
					}
					overrides[key] = contentType
				case "Default":
					ext, err := ooxmlAttribute(start, "Extension")
					key := ooxmlASCIIKey(ext)
					if err != nil || strings.ContainsAny(ext, "/\\.") || defaults[key] != "" {
						return errOOXMLInvalid
					}
					defaults[key] = contentType
				}
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	mainPart := ""
	relIDs := map[string]bool{}
	err = readOOXMLPart(ctx, relsFile, &remaining, func(r io.Reader) error {
		return walkOOXML(r, func(start xml.StartElement, depth int) error {
			if depth == 1 && start.Name != (xml.Name{Space: ooxmlRelsNS, Local: "Relationships"}) {
				return errOOXMLInvalid
			}
			if depth != 2 || start.Name != (xml.Name{Space: ooxmlRelsNS, Local: "Relationship"}) {
				return nil
			}
			id, err := ooxmlAttribute(start, "Id")
			if err != nil || relIDs[id] {
				return errOOXMLInvalid
			}
			relIDs[id] = true
			relType, err := ooxmlAttribute(start, "Type")
			if err != nil {
				return err
			}
			if relType != ooxmlMainRel && relType != ooxmlStrictMainRel {
				return nil // Обычные внешние гиперссылки не загружаем и не запрещаем.
			}
			if mainPart != "" {
				return errOOXMLInvalid
			}
			mode := ""
			for _, attr := range start.Attr {
				if attr.Name == (xml.Name{Local: "TargetMode"}) {
					if mode != "" {
						return errOOXMLInvalid
					}
					mode = attr.Value
				}
			}
			if mode != "" && mode != "Internal" {
				return errOOXMLInvalid
			}
			target, err := ooxmlAttribute(start, "Target")
			if err != nil {
				return err
			}
			mainPart, err = ooxmlRelationshipTarget(target)
			return err
		})
	})
	if err != nil {
		return err
	}
	mainFile := parts[mainPart]
	contentType := overrides[mainPart]
	if contentType == "" {
		contentType = defaults[ooxmlASCIIKey(strings.TrimPrefix(path.Ext(mainPart), "."))]
	}
	if mainPart == "" || mainFile == nil || mainFile == typesFile || mainFile == relsFile || contentType != format.contentType {
		return errOOXMLInvalid
	}
	for _, file := range archive.File {
		if file == typesFile || file == relsFile {
			continue // Каждую часть считаем и проверяем ровно один раз.
		}
		var consume func(io.Reader) error
		if file == mainFile {
			consume = func(r io.Reader) error {
				return walkOOXML(r, func(start xml.StartElement, depth int) error {
					if depth == 1 && (start.Name.Local != format.root || (start.Name.Space != format.namespace && start.Name.Space != format.strictNamespace)) {
						return errOOXMLInvalid
					}
					return nil
				})
			}
		}
		if err := readOOXMLPart(ctx, file, &remaining, consume); err != nil {
			return err
		}
	}
	return nil
}

// Бюджет уменьшается по Read, без суммы недоверенных UncompressedSize64.
type ooxmlBudgetReader struct {
	ctx       context.Context
	src       io.Reader
	remaining *int64
}

func (r ooxmlBudgetReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > 0 && int64(len(p))-1 > *r.remaining {
		p = p[:int(*r.remaining)+1]
	}
	n, err := r.src.Read(p)
	if int64(n) > *r.remaining {
		*r.remaining = 0
		return n, errOOXMLBudget
	}
	*r.remaining -= int64(n)
	return n, err
}

func readOOXMLPart(ctx context.Context, file *zip.File, remaining *int64, consume func(io.Reader) error) error {
	r, err := file.Open()
	if err != nil {
		return err
	}
	reader := ooxmlBudgetReader{ctx, r, remaining}
	if consume != nil {
		err = consume(reader)
	}
	if err == nil {
		_, err = io.Copy(io.Discard, reader) // EOF важен для CRC даже после XML-root.
	}
	return errors.Join(err, r.Close())
}

func ooxmlAttribute(start xml.StartElement, name string) (string, error) {
	value, found := "", false
	for _, attr := range start.Attr {
		if attr.Name == (xml.Name{Local: name}) {
			if found {
				return "", errOOXMLInvalid
			}
			value, found = attr.Value, true
		}
	}
	if !found || value == "" {
		return "", errOOXMLInvalid
	}
	return value, nil
}

func walkOOXML(r io.Reader, visit func(xml.StartElement, int) error) error {
	buffered := bufio.NewReader(r)
	prefix, _ := buffered.Peek(3)
	utf16 := len(prefix) >= 2 && ((prefix[0] == 0xff && prefix[1] == 0xfe) || (prefix[0] == 0xfe && prefix[1] == 0xff) || (prefix[0] == '<' && prefix[1] == 0) || (prefix[0] == 0 && prefix[1] == '<'))
	order := unicode.LittleEndian
	if utf16 && (prefix[0] == 0xfe || prefix[0] == 0) {
		order = unicode.BigEndian
	}
	var source io.Reader = buffered
	if utf16 {
		source = unicode.UTF16(order, unicode.UseBOM).NewDecoder().Reader(buffered)
	} else if bytes.HasPrefix(prefix, []byte{0xef, 0xbb, 0xbf}) {
		_, _ = buffered.Discard(3)
	}
	encodingMatches := func(label string) bool {
		switch strings.ToLower(label) {
		case "utf-8":
			return !utf16
		case "utf-16":
			return utf16
		case "utf-16le":
			return utf16 && order == unicode.LittleEndian
		case "utf-16be":
			return utf16 && order == unicode.BigEndian
		}
		return false
	}
	decoder := xml.NewDecoder(source)
	decoder.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		if encodingMatches(label) {
			return input, nil // Уже преобразовали байты, объявление не запускает второй decoder.
		}
		return nil, errOOXMLInvalid
	}
	depth, seenRoot := 0, false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if !seenRoot || depth != 0 {
				return errOOXMLInvalid
			}
			return nil
		}
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			attributes := make(map[xml.Name]bool, len(token.Attr))
			for _, attr := range token.Attr {
				if attributes[attr.Name] {
					return errOOXMLInvalid
				}
				attributes[attr.Name] = true
			}
			if depth == 0 {
				if seenRoot {
					return errOOXMLInvalid
				}
				seenRoot = true
			}
			depth++
			if err := visit(token, depth); err != nil {
				return err
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(token)) != 0 {
				return errOOXMLInvalid
			}
		case xml.ProcInst:
			if token.Target == "xml" {
				if depth != 0 || seenRoot {
					return errOOXMLInvalid
				}
				// encoding/xml пропускает CharsetReader для декларации UTF-8.
				// Поэтому сверяем также её с реально декодированной сериализацией.
				declaration := xml.NewDecoder(strings.NewReader("<declaration " + string(token.Inst) + "/>"))
				t, err := declaration.Token()
				start, ok := t.(xml.StartElement)
				if err != nil || !ok {
					return errOOXMLInvalid
				}
				found := false
				for _, attr := range start.Attr {
					if attr.Name == (xml.Name{Local: "encoding"}) {
						if found || !encodingMatches(attr.Value) {
							return errOOXMLInvalid
						}
						found = true
					}
				}
			}
		case xml.Directive:
			return errOOXMLInvalid // OPC запрещает DTD; внешние сущности не разрешаем.
		}
	}
}

func ooxmlASCIIKey(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func ooxmlPartName(name string, rootAllowed bool) (string, error) {
	if rootAllowed && strings.HasPrefix(name, "/") {
		name = name[1:]
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
		return "", errOOXMLInvalid
	}
	decoded, err := url.PathUnescape(name)
	if err != nil {
		return "", err
	}
	return ooxmlDecodedPartName(decoded)
}

func ooxmlDecodedPartName(name string) (string, error) {
	if !utf8.ValidString(name) || name == "" || strings.Contains(name, "\\") {
		return "", errOOXMLInvalid
	}
	for _, c := range name {
		if c < 0x20 || c == 0x7f {
			return "", errOOXMLInvalid
		}
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", errOOXMLInvalid
		}
	}
	return ooxmlASCIIKey(name), nil
}

func ooxmlRelationshipTarget(target string) (string, error) {
	u, err := url.Parse(target)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errOOXMLInvalid
	}
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") {
		return "", errOOXMLInvalid
	}
	name := strings.TrimPrefix(u.Path, "/")
	var segments []string
	for _, segment := range strings.Split(name, "/") {
		switch segment {
		case ".":
			continue
		case "..":
			if len(segments) == 0 {
				return "", errOOXMLInvalid
			}
			segments = segments[:len(segments)-1]
		default:
			segments = append(segments, segment)
		}
	}
	return ooxmlDecodedPartName(strings.Join(segments, "/"))
}
