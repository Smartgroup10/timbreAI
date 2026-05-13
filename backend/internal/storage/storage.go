package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thin S3-compatible object store wrapper. Targets MinIO via path-style addressing
// (single endpoint, optional TLS). Implements only what the recording flow needs: PutObject and
// a presigned-GET URL for downloads.
//
// We intentionally avoid pulling the AWS SDK; the surface area is small and the request signing
// for S3v4 is straightforward when we only need PUT + presigned GET.
type Client struct {
	Endpoint   string // e.g. http://minio:9000
	AccessKey  string
	SecretKey  string
	Region     string
	Bucket     string
	PublicURL  string // optional override used when generating presigned URLs (e.g. http://localhost:9000)
	httpClient *http.Client
}

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	PublicURL string
}

func New(cfg Config) *Client {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	return &Client{
		Endpoint:   strings.TrimRight(cfg.Endpoint, "/"),
		AccessKey:  cfg.AccessKey,
		SecretKey:  cfg.SecretKey,
		Region:     region,
		Bucket:     cfg.Bucket,
		PublicURL:  strings.TrimRight(cfg.PublicURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.Endpoint != "" && c.AccessKey != "" && c.Bucket != "" }

// EnsureBucket creates the bucket if it doesn't exist. Safe to call on every boot.
func (c *Client) EnsureBucket(ctx context.Context) error {
	if !c.Enabled() {
		return errors.New("storage_not_configured")
	}
	// HEAD bucket - 200 if exists, 404 otherwise.
	req, err := c.signedRequest(ctx, http.MethodHead, c.Bucket, "", nil, "")
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	// Create with PUT /bucket. MinIO accepts an empty body.
	req, err = c.signedRequest(ctx, http.MethodPut, c.Bucket, "", nil, "")
	if err != nil {
		return err
	}
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create bucket: %s: %s", resp.Status, string(body))
	}
	return nil
}

// PutObject uploads bytes under the given key inside the configured bucket. Returns the canonical
// object URL (not a presigned link).
func (c *Client) PutObject(ctx context.Context, key string, body []byte, contentType string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("storage_not_configured")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req, err := c.signedRequest(ctx, http.MethodPut, c.Bucket+"/"+key, "", body, contentType)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("put %s: %s: %s", key, resp.Status, string(b))
	}
	base := c.PublicURL
	if base == "" {
		base = c.Endpoint
	}
	return base + "/" + c.Bucket + "/" + key, nil
}

// PresignGet returns a presigned URL that can be opened in a browser without auth.
//
// Si PublicURL está definida (dev local o S3 público externo) se genera una URL
// absoluta contra ese host. Si NO está definida, se devuelve una URL relativa
// "/storage/{bucket}/{key}?…" pensada para que el frontend Next la proxee a
// MinIO interno (ver frontend/next.config.mjs). En ese caso la firma se calcula
// contra el host del Endpoint interno (minio:9000) — que es lo que verá MinIO
// cuando Next reenvíe la petición preservando query y headers.
func (c *Client) PresignGet(key string, ttl time.Duration) (string, error) {
	if !c.Enabled() {
		return "", errors.New("storage_not_configured")
	}
	relative := c.PublicURL == ""
	base := c.PublicURL
	if base == "" {
		base = c.Endpoint
	}
	u, err := url.Parse(base + "/" + c.Bucket + "/" + key)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	expires := int(ttl.Seconds())
	q := u.Query()
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	credential := c.AccessKey + "/" + now.Format("20060102") + "/" + c.Region + "/s3/aws4_request"
	q.Set("X-Amz-Credential", credential)
	q.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	q.Set("X-Amz-Expires", fmt.Sprintf("%d", expires))
	q.Set("X-Amz-SignedHeaders", "host")
	u.RawQuery = encodeQuery(q)

	canonReq := "GET\n" + u.Path + "\n" + u.RawQuery + "\nhost:" + u.Host + "\n\nhost\nUNSIGNED-PAYLOAD"
	scope := now.Format("20060102") + "/" + c.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + now.Format("20060102T150405Z") + "\n" + scope + "\n" + hexStr(sha256Sum([]byte(canonReq)))
	signingKey := derive(c.SecretKey, now.Format("20060102"), c.Region, "s3")
	signature := hexStr(hmacSHA256(signingKey, []byte(stringToSign)))

	q.Set("X-Amz-Signature", signature)
	u.RawQuery = encodeQuery(q)
	if relative {
		// "/storage" + "/<bucket>/<key>" + "?<query>"
		return "/storage" + u.Path + "?" + u.RawQuery, nil
	}
	return u.String(), nil
}

// --- helpers (signing) ---

func (c *Client) signedRequest(ctx context.Context, method, pathSegment, query string, body []byte, contentType string) (*http.Request, error) {
	endpoint, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}
	endpoint.Path = "/" + pathSegment
	if query != "" {
		endpoint.RawQuery = query
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := hexStr(sha256Sum(body))
	req.Header.Set("Host", endpoint.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonHeaders := "host:" + endpoint.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	canonRequest := method + "\n" + endpoint.Path + "\n" + req.URL.RawQuery + "\n" + canonHeaders + "\n" + signedHeaders + "\n" + payloadHash
	scope := dateStamp + "/" + c.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hexStr(sha256Sum([]byte(canonRequest)))
	signingKey := derive(c.SecretKey, dateStamp, c.Region, "s3")
	signature := hexStr(hmacSHA256(signingKey, []byte(stringToSign)))

	auth := "AWS4-HMAC-SHA256 Credential=" + c.AccessKey + "/" + scope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature
	req.Header.Set("Authorization", auth)
	return req, nil
}

func encodeQuery(v url.Values) string {
	// url.Values.Encode sorts by key — needed for canonical request.
	return v.Encode()
}
