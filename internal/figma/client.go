// Package figma provides a client for the Figma REST API.
package figma

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	BaseURL        = "https://api.figma.com/v1"
	DefaultTimeout = 30 * time.Second
)

// Client is a Figma REST API client.
type Client struct {
	httpClient  *http.Client
	accessToken string
	baseURL     string
}

// NewClient creates a new Figma API client.
func NewClient(accessToken string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		accessToken: accessToken,
		baseURL:     BaseURL,
	}
}

// WithTimeout sets a custom timeout for the client.
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	c.httpClient.Timeout = timeout
	return c
}

// doRequest performs an authenticated HTTP request.
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Figma-Token", c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &RateLimitError{
			RetryAfter: resp.Header.Get("Retry-After"),
		}
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Err != "" {
			return nil, &apiErr
		}
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetFile retrieves a Figma file by its key.
func (c *Client) GetFile(ctx context.Context, fileKey string, opts *GetFileOptions) (*File, error) {
	query := url.Values{}
	if opts != nil {
		if opts.Version != "" {
			query.Set("version", opts.Version)
		}
		if opts.Depth > 0 {
			query.Set("depth", fmt.Sprintf("%d", opts.Depth))
		}
		if opts.Geometry != "" {
			query.Set("geometry", opts.Geometry)
		}
		if opts.PluginData != "" {
			query.Set("plugin_data", opts.PluginData)
		}
		if opts.BranchData {
			query.Set("branch_data", "true")
		}
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/files/"+fileKey, query)
	if err != nil {
		return nil, err
	}

	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("parsing file response: %w", err)
	}

	return &file, nil
}

// GetFileNodes retrieves specific nodes from a Figma file.
func (c *Client) GetFileNodes(ctx context.Context, fileKey string, nodeIDs []string, opts *GetFileOptions) (*FileNodes, error) {
	query := url.Values{}
	query.Set("ids", strings.Join(nodeIDs, ","))

	if opts != nil {
		if opts.Version != "" {
			query.Set("version", opts.Version)
		}
		if opts.Depth > 0 {
			query.Set("depth", fmt.Sprintf("%d", opts.Depth))
		}
		if opts.Geometry != "" {
			query.Set("geometry", opts.Geometry)
		}
		if opts.PluginData != "" {
			query.Set("plugin_data", opts.PluginData)
		}
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/files/"+fileKey+"/nodes", query)
	if err != nil {
		return nil, err
	}

	var nodes FileNodes
	if err := json.Unmarshal(body, &nodes); err != nil {
		return nil, fmt.Errorf("parsing nodes response: %w", err)
	}

	return &nodes, nil
}

// GetImages exports images from a Figma file.
func (c *Client) GetImages(ctx context.Context, fileKey string, nodeIDs []string, opts *ImageExportOptions) (*ImageExport, error) {
	query := url.Values{}
	query.Set("ids", strings.Join(nodeIDs, ","))

	if opts != nil {
		if opts.Format != "" {
			query.Set("format", opts.Format)
		}
		if opts.Scale > 0 {
			query.Set("scale", fmt.Sprintf("%g", opts.Scale))
		}
		if opts.SVGIncludeID {
			query.Set("svg_include_id", "true")
		}
		if opts.SVGSimplifyStroke {
			query.Set("svg_simplify_stroke", "true")
		}
		if opts.UseAbsoluteBounds {
			query.Set("use_absolute_bounds", "true")
		}
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/images/"+fileKey, query)
	if err != nil {
		return nil, err
	}

	var export ImageExport
	if err := json.Unmarshal(body, &export); err != nil {
		return nil, fmt.Errorf("parsing images response: %w", err)
	}

	return &export, nil
}

// GetFileStyles retrieves styles from a Figma file.
func (c *Client) GetFileStyles(ctx context.Context, fileKey string) (*FileStyles, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/files/"+fileKey+"/styles", nil)
	if err != nil {
		return nil, err
	}

	var styles FileStyles
	if err := json.Unmarshal(body, &styles); err != nil {
		return nil, fmt.Errorf("parsing styles response: %w", err)
	}

	return &styles, nil
}

// GetFileComponents retrieves components from a Figma file.
func (c *Client) GetFileComponents(ctx context.Context, fileKey string) (*FileComponents, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/files/"+fileKey+"/components", nil)
	if err != nil {
		return nil, err
	}

	var components FileComponents
	if err := json.Unmarshal(body, &components); err != nil {
		return nil, fmt.Errorf("parsing components response: %w", err)
	}

	return &components, nil
}

// GetLocalVariables retrieves local variables from a Figma file.
func (c *Client) GetLocalVariables(ctx context.Context, fileKey string) (*LocalVariables, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/files/"+fileKey+"/variables/local", nil)
	if err != nil {
		return nil, err
	}

	var vars LocalVariables
	if err := json.Unmarshal(body, &vars); err != nil {
		return nil, fmt.Errorf("parsing variables response: %w", err)
	}

	return &vars, nil
}

// GetImageFills retrieves URLs for all image fills used in a Figma file.
// Returns a map of imageRef -> URL for all images used in fills, strokes, and backgrounds.
func (c *Client) GetImageFills(ctx context.Context, fileKey string) (map[string]string, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/files/"+fileKey+"/images", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Meta struct {
			Images map[string]string `json:"images"`
		} `json:"meta"`
		Error   bool   `json:"error"`
		Message string `json:"message,omitempty"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parsing image fills response: %w", err)
	}

	if response.Error {
		return nil, fmt.Errorf("API error: %s", response.Message)
	}

	return response.Meta.Images, nil
}

// DownloadImage downloads an image from a URL.
func (c *Client) DownloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
