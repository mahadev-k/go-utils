package yaml_configs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	prefix string
	values map[string]string
	err    error
	calls  [][]string
}

func (f *fakeResolver) Match(v string) bool { return strings.HasPrefix(v, f.prefix) }

func (f *fakeResolver) ResolveAll(_ context.Context, refs []string) (map[string]string, error) {
	f.calls = append(f.calls, refs)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]string{}
	for _, r := range refs {
		if v, ok := f.values[r]; ok {
			out[r] = v
		}
	}
	return out, nil
}

func TestResolve_YAMLAndEnvValues(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "db:\n  password: ref::db\n  host: plain\n  port: 5432\nalso: ref::db\nlist: [ref::db]\n")
	t.Setenv("APP_TOKEN", "ref::token")

	r := &fakeResolver{prefix: "ref::", values: map[string]string{"ref::db": "pw", "ref::token": "tok"}}
	cfg, err := Load(File(p), Env("APP_"), Resolve(r))
	require.NoError(t, err)

	assert.Equal(t, "pw", cfg.Get("db.password"))
	assert.Equal(t, "pw", cfg.Get("also"))
	assert.Equal(t, "tok", cfg.Get("token"))
	assert.Equal(t, "plain", cfg.Get("db.host"))
	assert.Equal(t, 5432, cfg.Get("db.port"))
	assert.Equal(t, []any{"ref::db"}, cfg.Get("list"), "list items are not resolved")

	require.Len(t, r.calls, 1, "one batch per resolver")
	assert.Equal(t, []string{"ref::db", "ref::token"}, r.calls[0], "distinct refs, sorted")
}

func TestResolve_OnlyEarlierSources(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "x: ref::x\n")
	r := &fakeResolver{prefix: "ref::", values: map[string]string{"ref::x": "X"}}

	cfg, err := Load(Resolve(r), File(p))
	require.NoError(t, err)
	assert.Equal(t, "ref::x", cfg.Get("x"))
	assert.Empty(t, r.calls, "no references loaded yet, resolver not called")
}

func TestResolve_FirstMatchingResolverWins(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "a: one::a\nb: two::b\n")
	r1 := &fakeResolver{prefix: "one::", values: map[string]string{"one::a": "A"}}
	r2 := &fakeResolver{prefix: "two::", values: map[string]string{"two::b": "B"}}

	cfg, err := Load(File(p), Resolve(r1, r2))
	require.NoError(t, err)
	assert.Equal(t, "A", cfg.Get("a"))
	assert.Equal(t, "B", cfg.Get("b"))
}

func TestResolve_Errors(t *testing.T) {
	d := t.TempDir()
	p := writeFile(t, d, "a.yaml", "secret: ref::missing\n")

	_, err := Load(File(p), Resolve(&fakeResolver{prefix: "ref::"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SECRET")
	assert.Contains(t, err.Error(), "ref::missing")

	boom := errors.New("access denied")
	_, err = Load(File(p), Resolve(&fakeResolver{prefix: "ref::", err: boom}))
	assert.ErrorIs(t, err, boom)
}
