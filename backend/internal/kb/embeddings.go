// Package kb implementa Retrieval-Augmented Generation por tenant.
//
// Pipeline:
//
//	upload doc → chunker → OpenAI embeddings → pgvector
//	bot tool call → query embedding → cosine search → top-K chunks
//
// El embedding model es text-embedding-3-small (1536 dim, $0.02/1M tok).
// Es el más barato de OpenAI y suficiente para retrieval de documentación
// corta. Si el cliente necesita más calidad puede pasarse a text-embedding-3-large
// (3072 dim) en una iteración futura — implica nueva tabla con vector(3072).
package kb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbeddingDim del modelo activo. Si cambias modelo, cambia también el
// schema (vector(N) en migration). text-embedding-3-small = 1536.
const EmbeddingDim = 1536

const embeddingsURL = "https://api.openai.com/v1/embeddings"
const embeddingsModel = "text-embedding-3-small"

// MaxBatchInputs límite que pone OpenAI por request. Si pasas más,
// rechaza con 400. Para ingest dividimos en batches; para queries siempre
// es 1 (el usuario hace una pregunta a la vez).
const MaxBatchInputs = 100

// EmbeddingsClient hace POST a OpenAI. Cliente con timeout amplio porque
// batches de 100 inputs pueden tardar ~3-5s en respuesta.
type EmbeddingsClient struct {
	apiKey string
	http   *http.Client
}

func NewEmbeddingsClient(apiKey string) *EmbeddingsClient {
	return &EmbeddingsClient{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Enabled devuelve true si hay api key disponible. Útil para que el
// handler de upload falle temprano con un error claro si el tenant
// no tiene credencial OpenAI configurada.
func (c *EmbeddingsClient) Enabled() bool { return c.apiKey != "" }

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
	} `json:"usage"`
}

// Embed devuelve un embedding por cada input, en el mismo orden.
// El campo Usage.PromptTokens permite contabilizar coste si más adelante
// queremos cobrarlo al tenant.
func (c *EmbeddingsClient) Embed(ctx context.Context, inputs []string) ([][]float32, int, error) {
	if !c.Enabled() {
		return nil, 0, errors.New("embeddings: api key missing")
	}
	if len(inputs) == 0 {
		return nil, 0, nil
	}
	if len(inputs) > MaxBatchInputs {
		return nil, 0, fmt.Errorf("embeddings: too many inputs (%d > %d)", len(inputs), MaxBatchInputs)
	}
	body, _ := json.Marshal(embeddingsRequest{Model: embeddingsModel, Input: inputs})
	req, err := http.NewRequestWithContext(ctx, "POST", embeddingsURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("embeddings: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("embeddings: %d: %s", resp.StatusCode, string(b))
	}
	var out embeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, 0, err
	}

	// OpenAI promete que data viene ordenado por index, pero defendemos
	// el invariante por si acaso.
	vecs := make([][]float32, len(inputs))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(inputs) {
			continue
		}
		vecs[d.Index] = d.Embedding
	}
	// Detectar gaps por si algún input se perdió silenciosamente.
	for i, v := range vecs {
		if v == nil {
			return nil, out.Usage.PromptTokens, fmt.Errorf("embeddings: missing vector for input %d", i)
		}
		if len(v) != EmbeddingDim {
			return nil, out.Usage.PromptTokens, fmt.Errorf("embeddings: unexpected dim %d (want %d)", len(v), EmbeddingDim)
		}
	}
	return vecs, out.Usage.PromptTokens, nil
}
