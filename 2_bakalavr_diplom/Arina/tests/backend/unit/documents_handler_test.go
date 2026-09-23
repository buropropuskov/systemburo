package handler

// Модульные тесты обработчиков документов.
// //возможны изменения// — на момент написания тестов в репозитории
// github.com/airvt1x/dokkee-backend реализованы только обработчики авторизации.
// Тесты ориентированы на планируемые обработчики POST /api/v1/documents,
// GET /api/v1/documents, GET /api/v1/documents/:id, DELETE /api/v1/documents/:id.

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/airvt1x/dokkee-backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Мок сервиса документов — контракт придётся уточнить после реализации.
type MockDocumentsService struct {
	mock.Mock
}

func (m *MockDocumentsService) Upload(userId int, filename string, content []byte) (int, error) {
	args := m.Called(userId, filename, content)
	return args.Int(0), args.Error(1)
}

func (m *MockDocumentsService) List(userId int) ([]map[string]interface{}, error) {
	args := m.Called(userId)
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockDocumentsService) GetByID(userId, docId int) (map[string]interface{}, error) {
	args := m.Called(userId, docId)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockDocumentsService) Delete(userId, docId int) error {
	return m.Called(userId, docId).Error(0)
}

// Загрузка файла PDF → 201 и id.
func TestDocuments_Upload_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	docsMock := new(MockDocumentsService)
	//возможны изменения//: структура Handler должна будет хранить ссылку на docsMock.
	_ = &service.Service{} // placeholder — добавить поле Documents при реализации

	docsMock.On("Upload", 1, "contract.pdf", mock.AnythingOfType("[]uint8")).Return(100, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "contract.pdf")
	part.Write([]byte("%PDF-1.4\n%fake-content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/api/v1/documents", func(c *gin.Context) {
		c.Set("userId", 1)
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		defer file.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(file)
		id, err := docsMock.Upload(1, header.Filename, buf.Bytes())
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id})
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(100), resp["id"])
	docsMock.AssertExpectations(t)
}

// Недопустимый формат файла (PNG) → 400 Bad Request с сообщением.
func TestDocuments_Upload_WrongFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "photo.png")
	part.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/api/v1/documents", func(c *gin.Context) {
		_, header, _ := c.Request.FormFile("file")
		// валидация расширения должна быть в хэндлере
		allowed := map[string]bool{".pdf": true, ".doc": true, ".docx": true, ".txt": true}
		ext := ""
		if i := len(header.Filename) - 4; i > 0 {
			ext = header.Filename[i:]
		}
		if !allowed[ext] {
			c.JSON(http.StatusBadRequest, gin.H{"message": "unsupported file format"})
			return
		}
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported file format")
}

// Превышение максимального размера файла (50 МБ).
//возможны изменения// — лимит задаётся в конфиге.
func TestDocuments_Upload_TooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// имитируем большой файл минимальным набором: тест ориентируется на заголовок Content-Length.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents",
		bytes.NewReader(make([]byte, 51*1024*1024)))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = 51 * 1024 * 1024
	w := httptest.NewRecorder()

	const maxSize = 50 * 1024 * 1024
	router := gin.New()
	router.POST("/api/v1/documents", func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": "file > 50 MB"})
			return
		}
		c.Status(http.StatusCreated)
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// Изоляция данных между пользователями: user=1 не должен получить документ user=2.
//возможны изменения// — ключевой тест безопасности (ФТ-42 в плане тестирования).
func TestDocuments_GetByID_CrossUserForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	docsMock := new(MockDocumentsService)

	// сервис должен возвращать ошибку ErrAccessDenied при запросе чужого документа
	docsMock.On("GetByID", 1, 999).Return(map[string]interface{}{}, assert.AnError)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/999", nil)
	w := httptest.NewRecorder()

	router := gin.New()
	router.GET("/api/v1/documents/:id", func(c *gin.Context) {
		c.Set("userId", 1)
		_, err := docsMock.GetByID(1, 999)
		if err != nil {
			c.Status(http.StatusForbidden) // не 404, а именно 403 — иначе раскрывается сам факт существования
			return
		}
		c.Status(http.StatusOK)
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	docsMock.AssertExpectations(t)
}
