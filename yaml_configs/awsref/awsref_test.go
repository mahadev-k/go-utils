package awsref

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yc "github.com/mahadev-k/go-utils/yaml_configs"
)

type fakeSecrets struct {
	mu      sync.Mutex
	secrets map[string]*string // nil value = binary secret
	calls   map[string]int
}

func (f *fakeSecrets) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := aws.ToString(in.SecretId)
	f.calls[id]++
	v, ok := f.secrets[id]
	if !ok {
		return nil, errors.New("ResourceNotFoundException")
	}
	if v == nil {
		return &secretsmanager.GetSecretValueOutput{SecretBinary: []byte{1}}, nil
	}
	return &secretsmanager.GetSecretValueOutput{SecretString: v}, nil
}

type fakeSSM struct {
	mu        sync.Mutex
	params    map[string]string
	calls     map[string]int
	decrypted bool
}

func (f *fakeSSM) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := aws.ToString(in.Name)
	f.calls[name]++
	f.decrypted = aws.ToBool(in.WithDecryption)
	v, ok := f.params[name]
	if !ok {
		return nil, errors.New("ParameterNotFound")
	}
	return &ssm.GetParameterOutput{Parameter: &ssmtypes.Parameter{Name: in.Name, Value: aws.String(v)}}, nil
}

func newFakes() (*fakeSecrets, *fakeSSM, *Resolver) {
	sm := &fakeSecrets{calls: map[string]int{}, secrets: map[string]*string{
		"prod/db": aws.String(`{"user":"admin","password":"p@ss$word","port":5432,"opts":{"ssl": true}}`),
		"plain":   aws.String("just-a-string"),
		"arn:aws:secretsmanager:ap-south-1:123:secret:app-AbC": aws.String(`{"k":"from-arn"}`),
		"binary": nil,
	}}
	ps := &fakeSSM{calls: map[string]int{}, params: map[string]string{
		"/app/origin-verify": "0123456789abcdef0123456789abcdef",
		"/app/key:3":         "v3",
	}}
	return sm, ps, &Resolver{Secrets: sm, SSM: ps}
}

func TestResolveAll(t *testing.T) {
	sm, ps, r := newFakes()
	refs := []string{
		"aws::secrets::prod/db$user",
		"aws::secrets::prod/db$password",
		"aws::secrets::prod/db$port",
		"aws::secrets::prod/db$opts",
		"aws::secrets::plain",
		"aws::secrets::arn:aws:secretsmanager:ap-south-1:123:secret:app-AbC$k",
		"aws::ssm::/app/origin-verify",
		"aws::ssm::/app/key:3",
	}
	out, err := r.ResolveAll(context.Background(), refs)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"aws::secrets::prod/db$user":     "admin",
		"aws::secrets::prod/db$password": "p@ss$word",
		"aws::secrets::prod/db$port":     "5432",
		"aws::secrets::prod/db$opts":     `{"ssl":true}`,
		"aws::secrets::plain":            "just-a-string",
		"aws::secrets::arn:aws:secretsmanager:ap-south-1:123:secret:app-AbC$k": "from-arn",
		"aws::ssm::/app/origin-verify":                                         "0123456789abcdef0123456789abcdef",
		"aws::ssm::/app/key:3":                                                 "v3",
	}, out)
	assert.Equal(t, 1, sm.calls["prod/db"], "secret fetched once for several keys")
	assert.True(t, ps.decrypted, "SSM WithDecryption")
}

func TestResolveAll_Errors(t *testing.T) {
	for ref, want := range map[string]string{
		"aws::secret::typo":            "unknown kind",
		"aws::secrets::":               "invalid reference",
		"aws::secrets::prod/db$":       "invalid reference",
		"aws::ssm::":                   "invalid reference",
		"aws::secrets::nope$k":         "ResourceNotFoundException",
		"aws::ssm::/nope":              "ParameterNotFound",
		"aws::secrets::prod/db$absent": `key "absent" not found`,
		"aws::secrets::plain$k":        "not a JSON object",
		"aws::secrets::binary":         "binary secrets are not supported",
	} {
		_, _, r := newFakes()
		_, err := r.ResolveAll(context.Background(), []string{ref})
		require.Error(t, err, ref)
		assert.Contains(t, err.Error(), want, ref)
		assert.NotContains(t, err.Error(), "just-a-string", "error must not leak secret content")
		assert.NotContains(t, err.Error(), "admin", "error must not leak secret content")
	}

	r := &Resolver{}
	_, err := r.ResolveAll(context.Background(), []string{"aws::ssm::/x", "aws::secrets::y"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no SSM client")
	assert.Contains(t, err.Error(), "no Secrets Manager client")
}

func TestWithLoad(t *testing.T) {
	_, _, r := newFakes()
	d := t.TempDir()
	p := filepath.Join(d, "env.yaml")
	require.NoError(t, os.WriteFile(p, []byte("db:\n  user: aws::secrets::prod/db$user\n  password: aws::secrets::prod/db$password\n"), 0o644))
	t.Setenv("APP_ORIGIN_VERIFY_SECRET", "aws::ssm::/app/origin-verify")
	t.Setenv("APP_TABLE_NAME", "sargam-dev")

	cfg, err := yc.LoadContext(context.Background(), yc.File(p), yc.Env("APP_"), yc.Resolve(r))
	require.NoError(t, err)

	assert.Equal(t, "admin", cfg.Get("db.user"))
	assert.Equal(t, "p@ss$word", cfg.Get("db.password"))
	assert.Equal(t, "0123456789abcdef0123456789abcdef", cfg.Get("ORIGIN_VERIFY_SECRET"))
	assert.Equal(t, "sargam-dev", cfg.Get("table.name"))

	t.Setenv("APP_BROKEN", "aws::ssm::/nope")
	_, err = yc.LoadContext(context.Background(), yc.File(p), yc.Env("APP_"), yc.Resolve(r))
	require.Error(t, err, "unresolvable reference fails the load")
}
