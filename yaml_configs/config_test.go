package yaml_configs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestLoad_OverridesAndDeepMerge(t *testing.T) {
	d := t.TempDir()
	base := writeFile(t, d, "env.yaml", "db:\n  host: base\n  port: 5432\n  pool:\n    max: 10\nname: app\n")
	over := writeFile(t, d, "env.local.yaml", "db:\n  port: 6000\n")

	cfg, err := Load(File(base), File(over))
	require.NoError(t, err)

	assert.Equal(t, "base", cfg.Get("db.host"))
	assert.Equal(t, 6000, cfg.Get("db.port"))
	assert.Equal(t, 10, cfg.Get("db.pool.max"))
	assert.Equal(t, "app", cfg.Get("name"))
}

func TestLoad_ScalarReplacesMapAndViceVersa(t *testing.T) {
	d := t.TempDir()
	base := writeFile(t, d, "a.yaml", "db:\n  host: h\n  port: 1\nmode: simple\n")
	over := writeFile(t, d, "b.yaml", "db: disabled\nmode:\n  kind: advanced\n")

	cfg, err := Load(File(base), File(over))
	require.NoError(t, err)

	assert.Equal(t, "disabled", cfg.Get("db"))
	assert.False(t, cfg.Has("db.host"))
	assert.False(t, cfg.Has("db.port"))
	assert.False(t, cfg.Has("mode"))
	assert.Equal(t, "advanced", cfg.Get("mode.kind"))
}

func TestLoad_NullUnsetsKey(t *testing.T) {
	d := t.TempDir()
	base := writeFile(t, d, "a.yaml", "token: abc\n")
	over := writeFile(t, d, "b.yaml", "token: ~\n")

	cfg, err := Load(File(base), File(over))
	require.NoError(t, err)
	assert.False(t, cfg.Has("token"))
	_, err = cfg.String("token")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestLoad_EmptyAndCommentOnlyFiles(t *testing.T) {
	d := t.TempDir()
	base := writeFile(t, d, "a.yaml", "x: 1\n")
	empty := writeFile(t, d, "b.yaml", "")
	comments := writeFile(t, d, "c.yaml", "# nothing here yet\n")

	cfg, err := Load(File(base), File(empty), File(comments))
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Get("x"))
}

func TestLoad_MissingFiles(t *testing.T) {
	d := t.TempDir()
	missing := filepath.Join(d, "nope.yaml")

	_, err := Load(File(missing))
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)

	cfg, err := Load(OptionalFile(missing))
	require.NoError(t, err)
	assert.Empty(t, cfg.Keys())
}

func TestLoad_InvalidYAML(t *testing.T) {
	d := t.TempDir()
	bad := writeFile(t, d, "bad.yaml", "db: [\n")
	_, err := Load(File(bad))
	require.Error(t, err)
	assert.Contains(t, err.Error(), bad)

	list := writeFile(t, d, "list.yaml", "- a\n- b\n")
	_, err = Load(File(list))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "top level must be a mapping")
}

func TestLoad_CollidingKeysIsAnError(t *testing.T) {
	d := t.TempDir()
	for _, content := range []string{
		"db:\n  Host: a\n  host: b\n",
		"db_port: 1\ndb:\n  port: 2\n",
	} {
		p := writeFile(t, d, "c.yaml", content)
		_, err := Load(File(p))
		require.Error(t, err, content)
		assert.Contains(t, err.Error(), "refer to the same setting")
	}
}

func TestLoad_ErrorDoesNotPoisonLaterLoads(t *testing.T) {
	d := t.TempDir()
	bad := writeFile(t, d, "bad.yaml", "db: [\n")
	good := writeFile(t, d, "good.yaml", "db:\n  port: 5432\n")

	_, err := Load(File(bad))
	require.Error(t, err)

	cfg, err := Load(File(good))
	require.NoError(t, err)
	assert.Equal(t, 5432, cfg.Get("db.port"))
}

func TestKeyNormalization(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "database:\n  max-conns: 7\n")
	cfg, err := Load(File(p))
	require.NoError(t, err)

	for _, k := range []string{"database.max-conns", "DATABASE_MAX_CONNS", "Database.Max_Conns", "database_max-conns"} {
		assert.Equal(t, 7, cfg.Get(k), k)
	}
	assert.Equal(t, []string{"DATABASE_MAX_CONNS"}, cfg.Keys())
}

func TestEnv_OverridesYAMLAndAddsKeys(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "db:\n  host: yamlhost\n  port: 5432\n")
	t.Setenv("MYAPP_DB_PORT", "9999")
	t.Setenv("MYAPP_ORIGIN_SECRET", "s3cr3t")
	t.Setenv("MYAPP_DB_HOST", "") // empty is ignored
	t.Setenv("OTHER_DB_PORT", "1")

	cfg, err := Load(File(p), Env("MYAPP_"))
	require.NoError(t, err)

	port, err := cfg.Int("db.port")
	require.NoError(t, err)
	assert.Equal(t, 9999, port)
	assert.Equal(t, "yamlhost", cfg.Get("db.host"))
	assert.Equal(t, "s3cr3t", cfg.Get("origin.secret"))
	assert.Equal(t, "s3cr3t", cfg.Get("ORIGIN_SECRET"))
	assert.False(t, cfg.Has("other.db.port"))
}

func TestEnv_PositionalPrecedence(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "port: 1\n")
	t.Setenv("APP_PORT", "2")

	cfg, err := Load(Env("APP_"), File(p))
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Get("port"), "file after Env wins")

	cfg, err = Load(File(p), Env("app_"))
	require.NoError(t, err)
	assert.Equal(t, "2", cfg.Get("port"), "Env after file wins; prefix is case-insensitive")
}

func TestEnv_Only(t *testing.T) {
	t.Setenv("SVC_TABLE_NAME", "tbl")
	t.Setenv("SVC_CORS_ALLOWED_ORIGINS", "https://a.example, https://b.example,")
	t.Setenv("SVC_TIMEOUT", "1m30s")
	t.Setenv("SVC_DEBUG", "true")

	cfg, err := Load(Env("SVC_"))
	require.NoError(t, err)

	s, err := cfg.String("table.name")
	require.NoError(t, err)
	assert.Equal(t, "tbl", s)

	origins, err := cfg.StringSlice("cors.allowed.origins")
	require.NoError(t, err)
	assert.Equal(t, []string{"https://a.example", "https://b.example"}, origins)

	d, err := cfg.Duration("timeout")
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, d)

	b, err := cfg.Bool("debug")
	require.NoError(t, err)
	assert.True(t, b)
}

func TestTypedGetters(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", `
port: 5432
ratio: 0.5
whole: 3.0
big: 18446744073709551615
enabled: true
name: app
timeout: 5s
hosts: [a, b, 3]
nested_list: [[a]]
`)
	cfg, err := Load(File(p))
	require.NoError(t, err)

	i64, err := cfg.Int64("port")
	require.NoError(t, err)
	assert.Equal(t, int64(5432), i64)

	s, err := cfg.String("port")
	require.NoError(t, err)
	assert.Equal(t, "5432", s)

	f, err := cfg.Float64("port")
	require.NoError(t, err)
	assert.Equal(t, 5432.0, f)

	n, err := cfg.Int("whole")
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	hosts, err := cfg.StringSlice("hosts")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "3"}, hosts)

	dur, err := cfg.Duration("timeout")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, dur)

	raw, err := GetAs[[]any](cfg, "hosts")
	require.NoError(t, err)
	assert.Len(t, raw, 3)

	for name, fn := range map[string]func() error{
		"int from fraction":   func() error { _, err := cfg.Int("ratio"); return err },
		"int64 overflow":      func() error { _, err := cfg.Int64("big"); return err },
		"bool from string":    func() error { _, err := cfg.Bool("name"); return err },
		"duration from int":   func() error { _, err := cfg.Duration("port"); return err },
		"string from list":    func() error { _, err := cfg.String("hosts"); return err },
		"slice of lists":      func() error { _, err := cfg.StringSlice("nested_list"); return err },
		"exact type mismatch": func() error { _, err := GetAs[map[string]int](cfg, "port"); return err },
	} {
		err := fn()
		assert.True(t, errors.Is(err, ErrInvalidValue), "%s: got %v", name, err)
	}

	_, err = cfg.String("missing")
	assert.ErrorIs(t, err, ErrNotFound)

	assert.Equal(t, 42, GetOr(cfg, "missing", 42))
	assert.Equal(t, 42, GetOr(cfg, "ratio", 42))
	assert.Equal(t, 5432, GetOr(cfg, "port", 42))
}

func TestNilConfigIsEmpty(t *testing.T) {
	var cfg *Config
	assert.Nil(t, cfg.Get("x"))
	assert.False(t, cfg.Has("x"))
	assert.Nil(t, cfg.Keys())
	_, err := cfg.String("x")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestLegacy_GetNeverPanics(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	assert.NotPanics(t, func() { assert.Equal(t, "", Get[string]("x")) }, "before load")

	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "port: 5432\nlist: [a, b]\n")
	_, err := LoadConfigWithOverrides(p)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		assert.Equal(t, int64(5432), Get[int64]("port"))
		assert.Equal(t, "5432", Get[string]("port"))
		assert.Equal(t, []string{"a", "b"}, Get[[]string]("list"))
		assert.Equal(t, 0, Get[int]("list"))
		assert.Equal(t, 5432, Get[int]("PORT"))
	})
}

func TestLegacy_ReloadAndErrorHandling(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	d := t.TempDir()
	bad := writeFile(t, d, "bad.yaml", "db: [\n")
	a := writeFile(t, d, "a.yaml", "v: 1\n")
	b := writeFile(t, d, "b.yaml", "v: 2\n")

	cfg, err := LoadConfigWithOverrides(bad)
	require.Error(t, err, "first load fails")
	assert.Nil(t, cfg)

	cfg, err = LoadConfigWithOverrides(a)
	require.NoError(t, err, "retry after a failure must actually load")
	assert.Equal(t, 1, cfg.Get("v"))
	assert.Equal(t, 1, Get[int]("v"))

	_, err = LoadConfigWithOverrides(b)
	require.NoError(t, err)
	assert.Equal(t, 2, Get[int]("v"), "second call with different paths reloads")

	_, err = LoadConfigWithOverrides(bad)
	require.Error(t, err)
	assert.Equal(t, 2, Get[int]("v"), "failed reload keeps previous config")
}
