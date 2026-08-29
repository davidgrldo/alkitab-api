package bible

// Catalog returns translations annotated with origin (local|proxy) and
// capabilities (chapter, and corpus when AllVerses succeeds for that id).
func (e *Engine) Catalog() []Translation {
	ts := e.src.Translations()
	out := make([]Translation, len(ts))
	copy(out, ts)
	for i := range out {
		out[i].Origin, out[i].Capabilities = describe(e.src, out[i].ID)
	}
	return out
}

func describe(src Source, id string) (origin string, caps []string) {
	caps = []string{"chapter"}
	origin = "proxy"
	for _, m := range sourceMembers(src) {
		if _, err := m.Books(id); err != nil {
			continue
		}
		if corp, ok := m.(Corpus); ok {
			origin = "local"
			if _, err := corp.AllVerses(id); err == nil {
				caps = append(caps, "corpus")
			}
		} else {
			origin = "proxy"
		}
		return origin, caps
	}
	return origin, caps
}

func sourceMembers(src Source) []Source {
	if c, ok := src.(*Chain); ok {
		return c.sources
	}
	return []Source{src}
}

// DefaultCorpusVersion returns the sole translation id that supports corpus
// ops, or ErrInvalidRef when zero or more than one apply.
func (e *Engine) DefaultCorpusVersion() (string, error) {
	var ids []string
	for _, t := range e.Catalog() {
		for _, c := range t.Capabilities {
			if c == "corpus" {
				ids = append(ids, t.ID)
				break
			}
		}
	}
	if len(ids) != 1 {
		return "", ErrInvalidRef
	}
	return ids[0], nil
}
