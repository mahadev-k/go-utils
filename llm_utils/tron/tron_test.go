package tron

import (
	"encoding/json"
	"strings"
	"testing"
)

func Test_BuildTRONDocument_SamplePeople(t *testing.T) {
	raw := `{
		"people": [
			{"name":"robin","age":20,"addresses":[{"city":"hawkins","zipcode":888212}]},
			{"name":"kaleb","age":15,"addresses":[{"city":"hawkins","zipcode":888212}]},
			{"name":"marco","age":25,"addresses":[{"city":"indiana","zipcode":223112}]}
		]
	}`
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	doc, err := BuildTRONDocument("people", payload)
	if err != nil {
		t.Fatalf("BuildCSVDocument: %v", err)
	}
	out := RenderTRONDocument(doc)

	expected := `age,name,addresses_id
20,robin,[1]
15,kaleb,[1]
25,marco,[2]
---addresses
city,zipcode
hawkins,888212
indiana,223112
`
	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Fatalf("render mismatch:\n got:\n%s\nwant:\n%s", out, expected)
	}
}

func Test_BuildTRONDocument_FourLevelNesting(t *testing.T) {
	raw := `{
		"root": [
			{"name":"a","kids":[{"name":"k1","pets":[{"type":"cat","shots":[{"name":"rabies"}]}]}]},
			{"name":"b","kids":[{"name":"k2","pets":[{"type":"dog","shots":[{"name":"rabies"},{"name":"distemper"}]}]}]}
		]
	}`
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	doc, err := BuildTRONDocument("root", payload)
	if err != nil {
		t.Fatalf("BuildTRONDocument: %v", err)
	}
	out := RenderTRONDocument(doc)

	expected := `name,kids_id
a,[1]
b,[2]
---kids
name,pets_id
k1,[1]
k2,[2]
---kids.pets
type,shots_id
cat,[1]
dog,[1 2]
---kids.pets.shots
name
rabies
distemper
`
	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Fatalf("render mismatch:\n got:\n%s\nwant:\n%s", out, expected)
	}
}

func Test_BuildTRONDocument_FourLevelEmbeddedObjects(t *testing.T) {
	raw := `{
		"root": [
			{"name":"top1","child":{"name":"c1","grand":{"name":"g1","great":{"value":1}}}},
			{"name":"top2","child":{"name":"c2","grand":{"name":"g2","great":{"value":2}}}}
		]
	}`
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	doc, err := BuildTRONDocument("root", payload)
	if err != nil {
		t.Fatalf("BuildTRONDocument: %v", err)
	}
	out := RenderTRONDocument(doc)

	expected := `name,child_id
top1,1
top2,2
---child
name,grand_id
c1,1
c2,2
---child.grand
name,great_id
g1,1
g2,2
---child.grand.great
value
1
2
`
	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Fatalf("render mismatch:\n got:\n%s\nwant:\n%s", out, expected)
	}
}

func Test_BuildTRONDocument_TenPeople_SharedAddressesCountries(t *testing.T) {
	raw := `{
		"people": [
			{"name":"p1","addresses":[{"street":"1st","city":"ny","country":{"name":"usa","code":"US"}}]},
			{"name":"p2","addresses":[{"street":"2nd","city":"ny","country":{"name":"usa","code":"US"}}]},
			{"name":"p3","addresses":[{"street":"1st","city":"ny","country":{"name":"usa","code":"US"}}]},
			{"name":"p4","addresses":[{"street":"3rd","city":"la","country":{"name":"usa","code":"US"}}]},
			{"name":"p5","addresses":[{"street":"2nd","city":"ny","country":{"name":"usa","code":"US"}}]},
			{"name":"p6","addresses":[{"street":"1st","city":"ny","country":{"name":"usa","code":"US"}}]},
			{"name":"p7","addresses":[{"street":"4th","city":"sf","country":{"name":"usa","code":"US"}}]},
			{"name":"p8","addresses":[{"street":"3rd","city":"la","country":{"name":"usa","code":"US"}}]},
			{"name":"p9","addresses":[{"street":"5th","city":"chi","country":{"name":"usa","code":"US"}}]},
			{"name":"p10","addresses":[{"street":"1st","city":"ny","country":{"name":"usa","code":"US"}}]}
		]
	}`

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	doc, err := BuildTRONDocument("people", payload)
	if err != nil {
		t.Fatalf("BuildTRONDocument: %v", err)
	}

	out := RenderTRONDocument(doc)

	expected := `name,addresses_id
p1,[1]
p2,[2]
p3,[1]
p4,[3]
p5,[2]
p6,[1]
p7,[4]
p8,[3]
p9,[5]
p10,[1]
---addresses
city,street,country_id
ny,1st,1
ny,2nd,1
la,3rd,1
sf,4th,1
chi,5th,1
---addresses.country
code,name
US,usa
`

	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Fatalf("render mismatch:\n got:\n%s\nwant:\n%s", out, expected)
	}
}
