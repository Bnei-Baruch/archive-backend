package es9

import (
	"embed"
	"encoding/json"
	"fmt"
)

// grammarMappingsFS embeds the standalone ES9 grammar index mappings (39
// languages), transformed once from the ES6 mappings. Embedding keeps the ES9
// grammar indexer self-contained (no dependency on es/mappings/ or a runtime
// data folder), which is required to eventually deprecate ES6.
//
//go:embed data/mappings/grammars/*.json
var grammarMappingsFS embed.FS

// GrammarMapping returns the ES9 grammar index mapping (settings + mappings) for
// a language, ready to pass to ES9Manager.CreateIndex.
func GrammarMapping(lang string) (map[string]interface{}, error) {
	b, err := grammarMappingsFS.ReadFile(fmt.Sprintf("data/mappings/grammars/grammars-%s.json", lang))
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
