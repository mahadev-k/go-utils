package yaml_configs

import (
	"context"
	"fmt"
	"sort"
)

// Resolver replaces reference values (e.g. "aws::ssm::/app/db-password")
// with the values they point to. See the awsref subpackage for AWS Secrets
// Manager and SSM Parameter Store.
type Resolver interface {
	// Match reports whether value is a reference this resolver handles.
	Match(value string) bool
	// ResolveAll resolves distinct references and returns ref -> value for
	// every ref given. Error messages must not contain resolved values.
	ResolveAll(ctx context.Context, refs []string) (map[string]string, error)
}

// Resolve resolves references in the string values loaded by the sources
// before it, so place it last:
//
//	yaml_configs.Load(yaml_configs.File("env.yaml"), yaml_configs.Env("APP_"), yaml_configs.Resolve(r))
//
// Each distinct reference is resolved once per Load. The first resolver whose
// Match returns true owns a value. Resolved values are not themselves
// resolved again. Values inside YAML lists are not resolved. Load fails if
// any reference cannot be resolved.
func Resolve(resolvers ...Resolver) Source { return resolveSource{resolvers: resolvers} }

type resolveSource struct{ resolvers []Resolver }

func (s resolveSource) apply(ctx context.Context, entries map[string]entry) error {
	// Group references by owning resolver; remember which keys use each ref.
	type pending struct {
		refs []string
		keys map[string][]string // ref -> config keys
	}
	work := make([]pending, len(s.resolvers))
	for key, e := range entries {
		v, ok := e.value.(string)
		if !ok {
			continue
		}
		for i, r := range s.resolvers {
			if !r.Match(v) {
				continue
			}
			p := &work[i]
			if p.keys == nil {
				p.keys = make(map[string][]string)
			}
			if _, seen := p.keys[v]; !seen {
				p.refs = append(p.refs, v)
			}
			p.keys[v] = append(p.keys[v], key)
			break
		}
	}

	resolved := make(map[string]string) // key -> value
	for i, r := range s.resolvers {
		p := work[i]
		if len(p.refs) == 0 {
			continue
		}
		sort.Strings(p.refs)
		values, err := r.ResolveAll(ctx, p.refs)
		if err != nil {
			return fmt.Errorf("resolve config references: %w", err)
		}
		for _, ref := range p.refs {
			val, ok := values[ref]
			if !ok {
				keys := p.keys[ref]
				sort.Strings(keys)
				return fmt.Errorf("resolve config references: %s: resolver returned no value for %q", keys[0], ref)
			}
			for _, key := range p.keys[ref] {
				resolved[key] = val
			}
		}
	}
	for key, val := range resolved {
		e := entries[key]
		e.value = val
		entries[key] = e
	}
	return nil
}
