package token_shrinker

import (
	"encoding/json"
	"fmt"
)

// ExampleBuildTShrinkDocument renders the sample payload into TShrink (CSV-like) tables.
func ExampleBuildTShrinkDocument() {
	raw := `{
  "people": [
    {"name": "robin","age": 20,"addresses": [{"city": "hawkins","zipcode": 888212}]},
    {"name": "kaleb","age": 15,"addresses": [{"city": "hawkins","zipcode": 888212}]},
    {"name": "marco","age": 25,"addresses": [{"city": "indiana","zipcode": 223112}]}
  ]
}`
	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(raw), &payload)

	doc, _ := BuildTShrinkDocument("people", payload)
	fmt.Print(RenderTShrinkDocument(doc))

	// Output:
	// name,age,addresses_id
	// robin,20,[1]
	// kaleb,15,[1]
	// marco,25,[2]
	// ---addresses
	// city,zipcode
	// hawkins,888212
	// indiana,223112
}
