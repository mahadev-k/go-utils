package examples

import (
	"fmt"
	"log"

	"github.com/mahadev-k/go-utils/llm_utils/tron"
)

func ExampleBuildTRONDocuments() {
	payload := []any{
		map[string]any{"name": "Alice", "age": 28, "city": "Seattle"},
		map[string]any{"name": "Bob", "age": 41, "city": "Boston"},
	}
	enc, err := tron.BuildTRONDocuments("root", payload)
	if err != nil {
		log.Fatalf("error building shrink document: %v", err)
	}

	fmt.Println(tron.RenderTRONDocument(enc))

	// Output:
	// age,city,name
	// 28,Seattle,Alice
	// 41,Boston,Bob
}
