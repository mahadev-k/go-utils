package tron

import (
	"encoding/json"
	"fmt"
)

// ExampleBuildTRONDocument renders the sample payload into TRON (CSV-like) tables.
func ExampleBuildTRONDocument() {
	raw := `{
  "people": [
    {"name": "robin","age": 20,"addresses": [{"city": "hawkins","zipcode": 888212}]},
    {"name": "kaleb","age": 15,"addresses": [{"city": "hawkins","zipcode": 888212}]},
    {"name": "marco","age": 25,"addresses": [{"city": "indiana","zipcode": 223112}]}
  ]
}`
	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(raw), &payload)

	doc, _ := BuildTRONDocument("people", payload)
	fmt.Print(RenderTRONDocument(doc))

	// Output:
	// age,name,addresses_id
	// 20,robin,[1]
	// 15,kaleb,[1]
	// 25,marco,[2]
	// ---addresses
	// city,zipcode
	// hawkins,888212
	// indiana,223112
}
