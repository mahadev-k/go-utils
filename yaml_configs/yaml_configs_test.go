package yaml_configs

import (
	"fmt"
	"log"
)

func ExampleLoadConfigWithOverrides() {
	// Load configs in order of precedence
	_, err := LoadConfigWithSuffix(
		"./test_data/env",
		"local",
	)
	if err != nil {
		log.Fatal(err)
	}

	// Values from later files override earlier ones
	fmt.Println(Get[string]("database.host"))
	fmt.Println(Get[int]("database.port"))
	fmt.Println(Get[string]("database.dbname"))
	fmt.Println(Get[string]("database.user"))
	fmt.Println(Get[string]("database.password"))
	// Output: localhost
	// 5430
	// postgres
	// postgres
	// postgres
}
