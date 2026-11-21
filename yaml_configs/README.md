# yaml_configs

Layered configuration from YAML files and environment variables. Sources are applied in order;
later sources override earlier ones. YAML files are deep-merged, so an override file only needs
the keys it changes.

```go
cfg, err := yaml_configs.Load(
	yaml_configs.File("config/env.yaml"),               // required
	yaml_configs.OptionalFile("config/env.local.yaml"), // skipped if missing
	yaml_configs.Env("MYAPP_"),                        // MYAPP_DATABASE_PORT overrides database.port
)
if err != nil {
	log.Fatal(err)
}

host, err := cfg.String("database.host")
port, err := cfg.Int("database.port")
timeout, err := cfg.Duration("http.timeout")        // "5s", "1m30s"
origins, err := cfg.StringSlice("cors.origins")     // YAML list or "a,b,c"
debug := yaml_configs.GetOr(cfg, "debug", false)    // default if unset or invalid
```

`LoadWithSuffix("config/env", "local", yaml_configs.Env("MYAPP_"))` is shorthand for
`env.yaml` (required) + `env.local.yaml` (optional) + extra sources.

**Keys** are case-insensitive and `.`, `-` and `_` are interchangeable: `database.port`,
`DATABASE_PORT` and `Database.Port` are the same key. That is how an environment variable
overrides a nested YAML key. Two keys in one file that normalize to the same name
(e.g. `Host` and `host`) are a load error.

**Environment variables** are read once, at `Load`. Only variables starting with the prefix
(case-insensitive) are used, with the prefix stripped; empty values are ignored. `Env("")` reads
the whole environment.

**Typed getters** (`String`, `Bool`, `Int`, `Int64`, `Float64`, `Duration`, `StringSlice`, or
generic `GetAs[T]`) parse strings, so values from env vars work, and return errors wrapping
`ErrNotFound` or `ErrInvalidValue`. They never panic. A YAML `null` counts as unset.

### Package-level API (deprecated)

`LoadConfigWithSuffix` / `LoadConfigWithOverrides` + `Get[T]` still work. Missing files are
skipped, each call reloads, and `Get[T]` returns the zero value instead of panicking when nothing
is loaded or the type doesn't match. `SetGlobal(cfg)` installs a config from `Load` for `Get[T]`.

```go
_, err := yaml_configs.LoadConfigWithSuffix("./test_data/env", "local")
host := yaml_configs.Get[string]("database.host")
```
