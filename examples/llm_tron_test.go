package examples

import (
	"fmt"
	"log"

	"github.com/mahadev-k/go-utils/llm_utils/token_shrinker"
)

func ExampleBuildTRONDocuments() {
	payload := []any{
		map[string]any{"name": "Alice", "age": 28, "city": "Seattle"},
		map[string]any{"name": "Bob", "age": 41, "city": "Boston"},
	}
	enc, err := token_shrinker.BuildTRONDocuments("root", payload)
	if err != nil {
		log.Fatalf("error building shrink document: %v", err)
	}

	fmt.Println(token_shrinker.RenderTRONDocument(enc))

	// Output:
	// name,age,city
	// Alice,28,Seattle
	// Bob,41,Boston
}
