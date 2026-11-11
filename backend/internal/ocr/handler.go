package ocr

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"

	_ "golang.org/x/image/webp"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

const (
	MaxImageBytes     = int64(5 << 20)
	MaxImagePixels    = int64(20_000_000)
	maxMultipartBytes = MaxImageBytes + (1 << 20)
)

var (
	ErrImageRequired    = errors.New("image field is required")
	ErrImageTooLarge    = errors.New("image exceeds 5 MiB")
	ErrInvalidMultipart = errors.New("invalid multipart request")
	ErrUnsupportedImage = errors.New("unsupported image format")
	ErrInvalidImage     = errors.New("invalid image")
	ErrTooManyPixels    = errors.New("image exceeds pixel limit")
	ErrMultipleImages   = errors.New("only one image is allowed")
)

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) Register(group *echo.Group) {
	group.POST("/ocr/recognize", h.Recognize)
}

func (h *Handler) Recognize(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
	}
	if principal.Role != auth.RoleTeacher && principal.Role != auth.RoleAdmin {
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	}
	request := c.Request()
	request.Body = http.MaxBytesReader(c.Response(), request.Body, maxMultipartBytes)
	reader, err := request.MultipartReader()
	if err != nil {
		return apperror.New(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "请求必须为multipart/form-data")
	}
	imageBytes, filename, err := readMultipartImage(reader)
	if err != nil {
		return imageError(err)
	}
	contentType, err := ValidateImage(imageBytes)
	if err != nil {
		return imageError(err)
	}
	text, err := h.client.Recognize(request.Context(), imageBytes, filename, contentType)
	if err != nil {
		return clientError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"text": text})
}

func readMultipartImage(reader *multipart.Reader) ([]byte, string, error) {
	var imageBytes []byte
	var filename string
	found := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				return nil, "", ErrImageTooLarge
			}
			return nil, "", ErrInvalidMultipart
		}
		if part.FormName() != "image" {
			_, err = io.Copy(io.Discard, part)
			part.Close()
			if err != nil {
				return nil, "", ErrInvalidMultipart
			}
			continue
		}
		if found {
			part.Close()
			return nil, "", ErrMultipleImages
		}
		found = true
		filename = part.FileName()
		imageBytes, err = io.ReadAll(io.LimitReader(part, MaxImageBytes+1))
		part.Close()
		if err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				return nil, "", ErrImageTooLarge
			}
			return nil, "", ErrInvalidMultipart
		}
		if int64(len(imageBytes)) > MaxImageBytes {
			return nil, "", ErrImageTooLarge
		}
	}
	if !found || len(imageBytes) == 0 {
		return nil, "", ErrImageRequired
	}
	return imageBytes, filename, nil
}

func ValidateImage(data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrImageRequired
	}
	if int64(len(data)) > MaxImageBytes {
		return "", ErrImageTooLarge
	}
	configuration, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", ErrInvalidImage
	}
	contentTypes := map[string]string{
		"jpeg": "image/jpeg",
		"png":  "image/png",
		"webp": "image/webp",
	}
	contentType, supported := contentTypes[format]
	if !supported {
		return "", ErrUnsupportedImage
	}
	if detected := http.DetectContentType(data); detected != contentType {
		return "", ErrUnsupportedImage
	}
	if configuration.Width <= 0 || configuration.Height <= 0 {
		return "", ErrInvalidImage
	}
	if int64(configuration.Width) > MaxImagePixels/int64(configuration.Height) {
		return "", ErrTooManyPixels
	}
	return contentType, nil
}

func imageError(err error) error {
	switch {
	case errors.Is(err, ErrImageTooLarge):
		return apperror.New(http.StatusRequestEntityTooLarge, "IMAGE_TOO_LARGE", "图片不能超过5MB")
	case errors.Is(err, ErrUnsupportedImage):
		return apperror.New(http.StatusUnsupportedMediaType, "UNSUPPORTED_IMAGE", "仅支持JPEG、PNG或WebP图片")
	case errors.Is(err, ErrTooManyPixels):
		return apperror.New(http.StatusUnprocessableEntity, "IMAGE_PIXELS_EXCEEDED", "图片不能超过2000万像素")
	case errors.Is(err, ErrInvalidImage):
		return apperror.New(http.StatusUnprocessableEntity, "INVALID_IMAGE", "图片无法解码")
	case errors.Is(err, ErrImageRequired):
		return apperror.New(http.StatusBadRequest, "IMAGE_REQUIRED", "必须上传一个非空image字段")
	case errors.Is(err, ErrMultipleImages):
		return apperror.New(http.StatusBadRequest, "MULTIPLE_IMAGES", "只能上传一张图片")
	default:
		return apperror.New(http.StatusBadRequest, "INVALID_MULTIPART", "multipart请求格式错误")
	}
}

func clientError(err error) error {
	switch {
	case errors.Is(err, ErrEmptyResult):
		return apperror.New(http.StatusUnprocessableEntity, "EMPTY_OCR_RESULT", "OCR未识别出文本")
	case errors.Is(err, ErrTimeout):
		return apperror.New(http.StatusGatewayTimeout, "OCR_TIMEOUT", "OCR服务超时")
	case errors.Is(err, ErrUnavailable):
		return apperror.New(http.StatusServiceUnavailable, "OCR_UNAVAILABLE", "OCR服务未就绪")
	case errors.Is(err, ErrUpstream):
		return apperror.New(http.StatusBadGateway, "OCR_UPSTREAM_ERROR", "OCR服务处理失败")
	default:
		return apperror.Internal(err)
	}
}
