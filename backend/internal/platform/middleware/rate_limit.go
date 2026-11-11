package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"school-exam-studio/backend/internal/platform/apperror"
)

const maxRateLimitBodyBytes = 64 << 10

var slidingWindowScript = redis.NewScript(`
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[1])
local clock = redis.call('TIME')
local now = clock[1] * 1000 + math.floor(clock[2] / 1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
local count = redis.call('ZCARD', KEYS[1])
if count >= limit then
  local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
  local retry = window
  if oldest[2] then
    retry = math.max(1, oldest[2] + window - now)
  end
  return {0, retry}
end
redis.call('ZADD', KEYS[1], now, ARGV[3])
redis.call('PEXPIRE', KEYS[1], window)
return {1, 0}
`)

type RateLimiter interface {
	Allow(context.Context, string) (bool, time.Duration, error)
}

type RedisRateLimiter struct {
	client redis.UniversalClient
	prefix string
	limit  int
	window time.Duration
}

func NewRedisRateLimiter(client redis.UniversalClient, prefix string, limit int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{client: client, prefix: prefix, limit: limit, window: window}
}

func (l *RedisRateLimiter) Allow(ctx context.Context, identity string) (bool, time.Duration, error) {
	if l.client == nil || l.limit <= 0 || l.window <= 0 {
		return false, 0, errors.New("invalid Redis rate limiter configuration")
	}
	digest := sha256.Sum256([]byte(identity))
	key := "rate_limit:" + l.prefix + ":" + hex.EncodeToString(digest[:])
	result, err := slidingWindowScript.Run(
		ctx,
		l.client,
		[]string{key},
		l.limit,
		l.window.Milliseconds(),
		uuid.NewString(),
	).Result()
	if err != nil {
		return false, 0, err
	}
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return false, 0, errors.New("unexpected Redis rate limiter response")
	}
	allowed, err := redisInteger(values[0])
	if err != nil {
		return false, 0, err
	}
	retryMilliseconds, err := redisInteger(values[1])
	if err != nil {
		return false, 0, err
	}
	return allowed == 1, time.Duration(retryMilliseconds) * time.Millisecond, nil
}

func redisInteger(value any) (int64, error) {
	switch number := value.(type) {
	case int64:
		return number, nil
	case string:
		return strconv.ParseInt(number, 10, 64)
	default:
		return 0, errors.New("unexpected Redis integer response")
	}
}

func LoginRateLimit(limiter RateLimiter) echo.MiddlewareFunc {
	return rateLimit(limiter, true)
}

func RegisterRateLimit(limiter RateLimiter) echo.MiddlewareFunc {
	return rateLimit(limiter, false)
}

func rateLimit(limiter RateLimiter, login bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if limiter == nil {
				return apperror.New(http.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "限流服务暂不可用")
			}
			body, err := readBody(c)
			if err != nil {
				return err
			}
			identity := requestIP(c.Request())
			if login {
				var credentials struct {
					SchoolCode string `json:"school_code"`
					Email      string `json:"email"`
				}
				if json.Unmarshal(body, &credentials) == nil {
					identity += "\x00" + credentials.SchoolCode + "\x00" +
						strings.ToLower(strings.TrimSpace(credentials.Email))
				}
			}
			allowed, retryAfter, err := limiter.Allow(c.Request().Context(), identity)
			if err != nil {
				return apperror.New(http.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "限流服务暂不可用")
			}
			if !allowed {
				seconds := int(math.Ceil(retryAfter.Seconds()))
				if seconds < 1 {
					seconds = 1
				}
				c.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
				return apperror.New(http.StatusTooManyRequests, "RATE_LIMITED", "请求过于频繁，请稍后重试")
			}
			return next(c)
		}
	}
}

func readBody(c echo.Context) ([]byte, error) {
	request := c.Request()
	if request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxRateLimitBodyBytes+1))
	if err != nil {
		return nil, apperror.New(http.StatusBadRequest, "INVALID_JSON", "无法读取请求内容")
	}
	_ = request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > maxRateLimitBodyBytes {
		return nil, apperror.New(http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "请求内容过大")
	}
	return body, nil
}

func requestIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}
