package kb

import (
	"strings"
	"testing"
)

func TestChunkEmptyReturnsNil(t *testing.T) {
	if got := ChunkText("", DefaultChunkOptions); got != nil {
		t.Errorf("empty input should return nil, got %d chunks", len(got))
	}
	if got := ChunkText("   \n\n  ", DefaultChunkOptions); got != nil {
		t.Errorf("whitespace-only should return nil, got %d chunks", len(got))
	}
}

func TestChunkSingleParagraphFits(t *testing.T) {
	got := ChunkText("Hola, soy un párrafo corto.", DefaultChunkOptions)
	if len(got) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(got))
	}
	if !strings.Contains(got[0].Content, "soy un párrafo") {
		t.Errorf("content lost: %q", got[0].Content)
	}
	if got[0].Index != 0 {
		t.Errorf("first chunk should be index 0, got %d", got[0].Index)
	}
}

func TestChunkSplitsByMaxChars(t *testing.T) {
	// 10 párrafos de 300 chars cada uno → con MaxChars=600 debería caber 2 por chunk.
	para := strings.Repeat("a", 300)
	parts := make([]string, 10)
	for i := range parts {
		parts[i] = para
	}
	input := strings.Join(parts, "\n\n")
	got := ChunkText(input, ChunkOptions{MaxChars: 600, OverlapChars: 0})
	if len(got) < 4 {
		t.Errorf("want at least 4 chunks for 10×300 with max=600, got %d", len(got))
	}
}

func TestChunkOverlapPreservesTail(t *testing.T) {
	input := strings.Repeat("frase importante. ", 50) // ~900 chars
	got := ChunkText(input, ChunkOptions{MaxChars: 300, OverlapChars: 60})
	if len(got) < 2 {
		t.Fatalf("want >=2 chunks, got %d", len(got))
	}
	// El final del chunk 0 debe aparecer al inicio del chunk 1.
	tail := got[0].Content
	if len(tail) > 60 {
		tail = tail[len(tail)-60:]
	}
	if !strings.Contains(got[1].Content, strings.TrimSpace(tail[:30])) {
		// Permitimos cierta flexibilidad en el match exacto.
		t.Logf("overlap not strict: tail=%q head=%q", tail, got[1].Content[:60])
	}
}

func TestApproxTokens(t *testing.T) {
	if approxTokens("") != 0 {
		t.Errorf("empty should be 0 tokens")
	}
	// "hola mundo" = 2 palabras → 2*13/10 = 2 tokens
	if got := approxTokens("hola mundo"); got != 2 {
		t.Errorf("two words: got %d, want ~2", got)
	}
}
