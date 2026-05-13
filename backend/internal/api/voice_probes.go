package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var errKeyMissing = errors.New("api_key_not_configured")

// pingOpenAI verifica una API key de OpenAI llamando a GET /v1/models. Endpoint
// barato, no consume créditos y devuelve 401 si la key es inválida.
func pingOpenAI(ctx context.Context, key string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	return doProbe(req, "openai")
}

// pingDeepgram pega a GET /v1/projects que requiere autenticación. 401 si la
// key es inválida; 200 si funciona.
func pingDeepgram(ctx context.Context, key string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.deepgram.com/v1/projects", nil)
	req.Header.Set("Authorization", "Token "+key)
	return doProbe(req, "deepgram")
}

// pingAssemblyAI pega a GET /v2/account, mismo patrón.
func pingAssemblyAI(ctx context.Context, key string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.assemblyai.com/v2/account", nil)
	req.Header.Set("Authorization", key)
	return doProbe(req, "assemblyai")
}

// doProbe ejecuta la request y mapea la respuesta a un error human-readable.
// No leemos el body del éxito; sí lo leemos parcialmente en errores para que
// el usuario vea por qué falló (mensaje del provider).
func doProbe(req *http.Request, name string) error {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s_unreachable: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%s_invalid_key (HTTP %d)", name, resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	return fmt.Errorf("%s HTTP %d: %s", name, resp.StatusCode, string(body))
}
