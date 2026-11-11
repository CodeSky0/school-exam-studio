package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultTimeout         = 60 * time.Second
	DefaultMaxConcurrent   = 4
	DefaultMaxResponseSize = int64(1 << 20)
)

var (
	ErrUnavailable = errors.New("OCR sidecar unavailable")
	ErrTimeout     = errors.New("OCR sidecar timeout")
	ErrUpstream    = errors.New("OCR sidecar failed")
	ErrEmptyResult = errors.New("OCR returned empty text")
)

type Client struct {
	endpoint         string
	httpClient       *http.Client
	timeout          time.Duration
	slots            chan struct{}
	maxResponseBytes int64
}

func NewClient(baseURL string, timeout time.Duration, maxConcurrent int) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("OCR URL must be an absolute HTTP(S) URL without query or fragment")
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrent
	}
	dialTimeout := timeout
	if dialTimeout > 10*time.Second {
		dialTimeout = 10 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = dialTimeout
	transport.ResponseHeaderTimeout = timeout
	transport.MaxIdleConns = maxConcurrent * 2
	transport.MaxIdleConnsPerHost = maxConcurrent
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/recognize"
	return &Client{
		endpoint: parsed.String(),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		timeout:          timeout,
		slots:            make(chan struct{}, maxConcurrent),
		maxResponseBytes: DefaultMaxResponseSize,
	}, nil
}

func (c *Client) Recognize(ctx context.Context, image []byte, filename, contentType string) (string, error) {
	requestContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-requestContext.Done():
		return "", timeoutError(requestContext.Err())
	}
	if len(image) == 0 {
		return "", fmt.Errorf("%w: empty request image", ErrUpstream)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": "image", "filename": filename})
	if disposition == "" {
		disposition = `form-data; name="image"; filename="image"`
	}
	header.Set("Content-Disposition", disposition)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(image); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, c.endpoint, &body)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", requestError(err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusServiceUnavailable {
		return "", fmt.Errorf("%w: upstream status %d", ErrUnavailable, response.StatusCode)
	}
	if response.StatusCode == http.StatusUnprocessableEntity {
		errorBody, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		if readErr == nil {
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(errorBody, &envelope) == nil &&
				(envelope.Error.Code == "EMPTY_RECOGNITION" || envelope.Error.Code == "EMPTY_OCR_RESULT") {
				return "", ErrEmptyResult
			}
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("%w: upstream status %d", ErrUpstream, response.StatusCode)
	}
	limited := io.LimitReader(response.Body, c.maxResponseBytes+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", ErrTimeout
		}
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return "", ErrTimeout
		}
		return "", fmt.Errorf("%w: read response: %v", ErrUpstream, err)
	}
	if int64(len(responseBody)) > c.maxResponseBytes {
		return "", fmt.Errorf("%w: response exceeds %d bytes", ErrUpstream, c.maxResponseBytes)
	}
	var result struct {
		Text string `json:"text"`
	}
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return "", fmt.Errorf("%w: invalid JSON response", ErrUpstream)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("%w: multiple JSON values", ErrUpstream)
	}
	if strings.TrimSpace(result.Text) == "" {
		return "", ErrEmptyResult
	}
	return result.Text, nil
}

func timeoutError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	return err
}

func requestError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return ErrTimeout
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}
