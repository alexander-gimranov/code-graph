package embed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"codegraphindexer/internal/config"
)

const batchSize = 64

type Client struct {
	cfg    config.Embed
	client *http.Client
}

func New(cfg config.Embed) *Client {
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	if c.cfg.Provider == "" {
		return false
	}
	if c.cfg.Provider == "ollama" {
		return true
	}
	return c.cfg.Key != ""
}

func L2Norm(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

func Cosine(a, b []float64, normA, normB float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (normA * normB)
}

// EmbedOne returns a single embedding vector.
func (c *Client) EmbedOne(text string) ([]float64, error) {
	vecs, err := c.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 || vecs[0] == nil {
		return nil, fmt.Errorf("empty embedding")
	}
	return vecs[0], nil
}

// EmbedBatch embeds texts. Voyage/OpenAI use batches; Ollama is one-by-one.
func (c *Client) EmbedBatch(texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	switch c.cfg.Provider {
	case "voyage":
		return c.batchHTTP(texts, c.voyageBatch)
	case "openai":
		return c.batchHTTP(texts, c.openaiBatch)
	case "ollama":
		out := make([][]float64, len(texts))
		for i, t := range texts {
			v, err := c.ollamaOne(t)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown embed provider: %s", c.cfg.Provider)
	}
}

func (c *Client) batchHTTP(texts []string, fn func([]string) ([][]float64, error)) ([][]float64, error) {
	out := make([][]float64, 0, len(texts))
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		part, err := fn(texts[i:end])
		if err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	return out, nil
}

func (c *Client) voyageBatch(texts []string) ([][]float64, error) {
	body := map[string]any{"input": texts, "model": "voyage-code-3"}
	raw, err := c.postJSON("https://api.voyageai.com/v1/embeddings", map[string]string{
		"Authorization": "Bearer " + c.cfg.Key,
		"Content-Type":  "application/json",
	}, body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	out := make([][]float64, len(texts))
	for _, d := range resp.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	return out, nil
}

func (c *Client) openaiBatch(texts []string) ([][]float64, error) {
	body := map[string]any{"input": texts, "model": "text-embedding-3-small"}
	raw, err := c.postJSON("https://api.openai.com/v1/embeddings", map[string]string{
		"Authorization": "Bearer " + c.cfg.Key,
		"Content-Type":  "application/json",
	}, body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	out := make([][]float64, len(texts))
	for _, d := range resp.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	return out, nil
}

func (c *Client) ollamaOne(text string) ([]float64, error) {
	url := c.cfg.OllamaURL
	if url == "" {
		url = "http://localhost:11434"
	}
	model := c.cfg.OllamaModel
	if model == "" {
		model = "nomic-embed-text"
	}
	raw, err := c.postJSON(url+"/api/embeddings", map[string]string{
		"Content-Type": "application/json",
	}, map[string]any{"model": model, "prompt": text})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return resp.Embedding, nil
}

func (c *Client) postJSON(url string, headers map[string]string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
