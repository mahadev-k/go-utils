// Package awsref resolves config values that reference AWS Secrets Manager
// secrets and SSM Parameter Store parameters. Use it with yaml_configs.Resolve:
//
//	awsCfg, _ := config.LoadDefaultConfig(ctx)
//	cfg, err := yaml_configs.LoadContext(ctx,
//		yaml_configs.File("env.yaml"),
//		yaml_configs.Env("APP_"),
//		yaml_configs.Resolve(awsref.New(awsCfg)),
//	)
//
// Reference formats:
//
//	aws::secrets::<secret-id>         whole SecretString
//	aws::secrets::<secret-id>$<key>   one field of a JSON SecretString
//	aws::ssm::<parameter-name>        parameter value (SecureString is decrypted)
//
// <secret-id> is a secret name or ARN; <parameter-name> is a name, ARN, or
// name with a version/label selector ("/app/key:3"). Each distinct secret or
// parameter is fetched once per load, concurrently.
//
// Any value starting with "aws::" is treated as a reference, so a typo such as
// "aws::secret::x" fails the load instead of leaking through as a literal.
//
// Required IAM: secretsmanager:GetSecretValue, ssm:GetParameter, and
// kms:Decrypt for customer-managed KMS keys.
package awsref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	Prefix        = "aws::"
	SecretsPrefix = Prefix + "secrets::"
	SSMPrefix     = Prefix + "ssm::"
)

// SecretsManagerAPI is the subset of *secretsmanager.Client used here.
type SecretsManagerAPI interface {
	GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// SSMAPI is the subset of *ssm.Client used here.
type SSMAPI interface {
	GetParameter(ctx context.Context, in *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// Resolver implements yaml_configs.Resolver. A nil client makes references
// of that kind fail.
type Resolver struct {
	Secrets SecretsManagerAPI
	SSM     SSMAPI
	// Concurrency caps parallel AWS calls; <= 0 means 8.
	Concurrency int
}

// New returns a Resolver with Secrets Manager and SSM clients built from cfg.
func New(cfg aws.Config) *Resolver {
	return &Resolver{Secrets: secretsmanager.NewFromConfig(cfg), SSM: ssm.NewFromConfig(cfg)}
}

// Match reports whether v is an aws:: reference.
func (r *Resolver) Match(v string) bool { return strings.HasPrefix(v, Prefix) }

type ref struct {
	raw    string
	kind   string // "secrets" | "ssm"
	id     string // secret id or parameter name
	key    string // JSON field, secrets only
	hasKey bool
}

func parse(raw string) (ref, error) {
	switch {
	case strings.HasPrefix(raw, SecretsPrefix):
		rest := strings.TrimPrefix(raw, SecretsPrefix)
		// Secret names and ARNs cannot contain '$', so the first '$' splits.
		id, key, hasKey := strings.Cut(rest, "$")
		if id == "" || (hasKey && key == "") {
			return ref{}, fmt.Errorf("invalid reference %q: want aws::secrets::<secret-id>[$<key>]", raw)
		}
		return ref{raw: raw, kind: "secrets", id: id, key: key, hasKey: hasKey}, nil
	case strings.HasPrefix(raw, SSMPrefix):
		name := strings.TrimPrefix(raw, SSMPrefix)
		if name == "" {
			return ref{}, fmt.Errorf("invalid reference %q: want aws::ssm::<parameter-name>", raw)
		}
		return ref{raw: raw, kind: "ssm", id: name}, nil
	}
	return ref{}, fmt.Errorf("invalid reference %q: unknown kind, want aws::secrets:: or aws::ssm::", raw)
}

// ResolveAll implements yaml_configs.Resolver.
func (r *Resolver) ResolveAll(ctx context.Context, raws []string) (map[string]string, error) {
	refs := make([]ref, 0, len(raws))
	type fetchKey struct{ kind, id string }
	var fetches []fetchKey
	seen := make(map[fetchKey]bool)
	for _, raw := range raws {
		rf, err := parse(raw)
		if err != nil {
			return nil, err
		}
		refs = append(refs, rf)
		fk := fetchKey{rf.kind, rf.id}
		if !seen[fk] {
			seen[fk] = true
			fetches = append(fetches, fk)
		}
	}

	limit := r.Concurrency
	if limit <= 0 {
		limit = 8
	}
	var (
		mu      sync.Mutex
		fetched = make(map[fetchKey]string, len(fetches))
		errs    []error
		wg      sync.WaitGroup
		sem     = make(chan struct{}, limit)
	)
	for _, fk := range fetches {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			var v string
			var err error
			if fk.kind == "secrets" {
				v, err = r.getSecret(ctx, fk.id)
			} else {
				v, err = r.getParameter(ctx, fk.id)
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			fetched[fk] = v
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	out := make(map[string]string, len(refs))
	for _, rf := range refs {
		v := fetched[fetchKey{rf.kind, rf.id}]
		if rf.hasKey {
			field, err := jsonField(v, rf.key)
			if err != nil {
				return nil, fmt.Errorf("secret %s: %w", rf.id, err)
			}
			v = field
		}
		out[rf.raw] = v
	}
	return out, nil
}

func (r *Resolver) getSecret(ctx context.Context, id string) (string, error) {
	if r.Secrets == nil {
		return "", fmt.Errorf("secret %s: no Secrets Manager client configured", id)
	}
	out, err := r.Secrets.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(id)})
	if err != nil {
		return "", fmt.Errorf("secret %s: %w", id, err)
	}
	if out.SecretString == nil {
		return "", fmt.Errorf("secret %s: binary secrets are not supported", id)
	}
	return *out.SecretString, nil
}

func (r *Resolver) getParameter(ctx context.Context, name string) (string, error) {
	if r.SSM == nil {
		return "", fmt.Errorf("ssm parameter %s: no SSM client configured", name)
	}
	out, err := r.SSM.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(name), WithDecryption: aws.Bool(true)})
	if err != nil {
		return "", fmt.Errorf("ssm parameter %s: %w", name, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", fmt.Errorf("ssm parameter %s: empty response", name)
	}
	return *out.Parameter.Value, nil
}

// jsonField extracts key from a JSON object. String fields are returned
// unquoted; other JSON values are returned as compact JSON text. Errors never
// include the secret's content.
func jsonField(secret, key string) (string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(secret), &obj); err != nil {
		return "", fmt.Errorf("key %q requested but secret value is not a JSON object", key)
	}
	raw, ok := obj[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", fmt.Errorf("key %q: invalid JSON value", key)
	}
	return buf.String(), nil
}
