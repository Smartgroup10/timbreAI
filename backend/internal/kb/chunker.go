package kb

import "strings"

// Chunk es un fragmento de texto listo para embedding.
type Chunk struct {
	Index   int
	Content string
	Tokens  int // estimación grosera por palabras; suficiente para mostrar tamaño
}

// ChunkOptions controla la granularidad. Defaults: MaxChars=2000 (~500
// tokens), OverlapChars=200. El overlap evita perder contexto justo en
// los bordes — si la respuesta cae entre dos chunks, sigue siendo
// recuperable porque el solape se replica.
type ChunkOptions struct {
	MaxChars     int
	OverlapChars int
}

// DefaultChunkOptions: 2000 chars con 200 de solape (~10%). Buen
// compromiso para documentación corporativa, FAQs e immobles.
var DefaultChunkOptions = ChunkOptions{MaxChars: 2000, OverlapChars: 200}

// Chunk divide un texto en pedazos respetando límites naturales:
//
//  1. Normaliza saltos de línea.
//  2. Divide por párrafos (línea en blanco entre bloques).
//  3. Acumula párrafos hasta llegar a MaxChars.
//  4. Si un párrafo solo es más largo que MaxChars, lo trocea por puntos
//     y, si aún así no entra, hard split por longitud.
//  5. Añade Overlap del final del chunk anterior al inicio del siguiente
//     para preservar contexto.
//
// Garantías:
//   - Cada chunk tiene contenido no vacío.
//   - El índice es secuencial (0, 1, 2, ...).
//   - Total de chunks suma >= len(input) - Overlap*chunks (con solape).
func ChunkText(text string, opt ChunkOptions) []Chunk {
	if opt.MaxChars <= 0 {
		opt = DefaultChunkOptions
	}
	if opt.OverlapChars < 0 || opt.OverlapChars >= opt.MaxChars {
		opt.OverlapChars = 0
	}
	text = normalize(text)
	if text == "" {
		return nil
	}

	paragraphs := splitParagraphs(text)
	// Trocear párrafos demasiado largos a nivel oración.
	expanded := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		if len(p) <= opt.MaxChars {
			expanded = append(expanded, p)
			continue
		}
		expanded = append(expanded, splitOversized(p, opt.MaxChars)...)
	}

	out := []Chunk{}
	var buf strings.Builder
	flush := func() {
		s := strings.TrimSpace(buf.String())
		if s == "" {
			return
		}
		out = append(out, Chunk{
			Index:   len(out),
			Content: s,
			Tokens:  approxTokens(s),
		})
		// Reset con overlap: arrastra los últimos OverlapChars al siguiente.
		if opt.OverlapChars > 0 && len(s) > opt.OverlapChars {
			tail := s[len(s)-opt.OverlapChars:]
			buf.Reset()
			buf.WriteString(tail)
			buf.WriteString("\n")
		} else {
			buf.Reset()
		}
	}
	for _, p := range expanded {
		if buf.Len()+len(p)+1 > opt.MaxChars && buf.Len() > 0 {
			flush()
		}
		buf.WriteString(p)
		buf.WriteString("\n\n")
	}
	flush()
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalize(s string) string {
	// Unifica CRLF/CR a LF. Recorta trailing whitespace en líneas.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

func splitParagraphs(s string) []string {
	// Línea en blanco como separador de párrafo. Si no hay líneas en
	// blanco, todo el texto es un único párrafo (cae al splitOversized
	// después).
	raw := strings.Split(s, "\n\n")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitOversized trocea un párrafo demasiado largo por oraciones
// (separadas por punto+espacio). Si una oración también excede, hard
// split por longitud.
func splitOversized(p string, max int) []string {
	sentences := splitSentences(p)
	var out []string
	var buf strings.Builder
	flush := func() {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			out = append(out, s)
		}
		buf.Reset()
	}
	for _, s := range sentences {
		if len(s) > max {
			// Hard split: nada más se puede hacer sin perder boundaries.
			flush()
			for len(s) > max {
				out = append(out, s[:max])
				s = s[max:]
			}
			if s != "" {
				buf.WriteString(s)
				buf.WriteString(" ")
			}
			continue
		}
		if buf.Len()+len(s)+1 > max && buf.Len() > 0 {
			flush()
		}
		buf.WriteString(s)
		buf.WriteString(" ")
	}
	flush()
	return out
}

func splitSentences(s string) []string {
	// Tokenizer de pacotilla: split por ". " " ! " " ? " preservando
	// el carácter terminal. Suficiente para texto corporativo en
	// castellano/inglés. NO maneja abreviaturas tipo "Sr." — daría
	// false positives pero el chunk resultante sigue siendo válido.
	var out []string
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		buf.WriteByte(s[i])
		if (s[i] == '.' || s[i] == '!' || s[i] == '?') && i+1 < len(s) && s[i+1] == ' ' {
			out = append(out, buf.String())
			buf.Reset()
			i++ // skip the space
		}
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

// approxTokens estima tokens contando palabras × 1.3 (regla de pulgar
// para inglés/español con tokenizer BPE). Para mostrar tamaño en UI.
func approxTokens(s string) int {
	words := strings.Fields(s)
	return (len(words) * 13) / 10
}
