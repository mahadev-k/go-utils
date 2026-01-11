# TRON

TRON utilities for LLM-bound JSON payloads using a simple, CSV-like columnar format (“TRON”). Nested objects become separate tables; parents reference children with `<field>_id` (1-based). Strings are not pooled (so the output is readable and LLM-friendly), and ordering is deterministic (`name` first when present, then alpha).

## Quick example: flatten JSON to TRON tables

Given a payload:
```json
{
  "people": [
    {"name": "robin","age": 20,"addresses": [{"city": "hawkins","zipcode": 888212}]},
    {"name": "kaleb","age": 15,"addresses": [{"city": "hawkins","zipcode": 888212}]},
    {"name": "marco","age": 25,"addresses": [{"city": "indiana","zipcode": 223112}]}
  ]
}
```

```go
var payload map[string]interface{}
_ = json.Unmarshal(rawJSONBytes, &payload)

doc, err := token_shrinker.BuildTRONDocument("people", payload)
if err != nil {
    log.Fatal(err)
}

fmt.Print(token_shrinker.RenderTRONDocument(doc))
```

Output:
```
name,age,addresses_id
robin,20,[1]
kaleb,15,[1]
marco,25,[2]
---addresses
city,zipcode
hawkins,888212
indiana,223112
```

## Notes
- Root key must point to an array (e.g., `people` in the example).
- Child tables are named `<parent>.<field>` and rendered with the parent prefix stripped (e.g., `---addresses`).
- Arrays of objects become child tables; arrays of scalars are stored inline.
- A small string-pool encoder/decoder still exists in history, but TRON is the recommended path for token-lean, human-inspectable payloads.
