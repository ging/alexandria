// Package app implements the Rules use case: CRUD with If-Match on the revision, whole-set validation, persistence, atomic swap and vhub.rules.
// The seed is loaded only when the store is empty; a rejected change never alters the current set (§4.3.2, REQ-CLI-028, REQ-CLI-029, REQ-CLI-031).
package app

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/timex"
)

// Rules manages the rule set.
type Rules struct {
	Store         RuleStore
	Cell          *RulesCell
	Events        EventSink
	Clock         timex.Clock
	Logger        *slog.Logger
	Namespace     string
	HasManagement bool
	mu            sync.Mutex
}

// ETag renders the If-Match token of a revision.
func ETag(rev int) string { return `"` + strconv.Itoa(rev) + `"` }

// Init loads the persisted set or, with an empty store, the seed file (CFG-2).
func (r *Rules) Init(seed string) *codes.Error {
	rs, ok, err := r.Store.Load()
	if err != nil {
		return codes.New(codes.ConfigRulesSeedUnreadable, "rule store: %v", err)
	}
	if !ok {
		rs = &domain.Ruleset{Revision: 0, Rules: []domain.Rule{}}
		if seed != "" {
			b, err := os.ReadFile(strings.TrimPrefix(strings.TrimPrefix(seed, "file://"), "file:")) // #nosec G304 -- operator-chosen seed path.
			if err != nil {
				return codes.New(codes.ConfigRulesSeedUnreadable, "%s: %v", seed, err)
			}
			parsed, cerr := domain.ParseRuleset(b)
			if cerr != nil {
				return cerr
			}
			rs = parsed
		}
		if cerr := rs.Check(r.HasManagement); cerr != nil {
			return cerr
		}
		if err := r.Store.Save(rs); err != nil {
			return codes.New(codes.ConfigRulesSeedUnreadable, "rule store: %v", err)
		}
	}
	r.Cell.Store(rs)
	logx.Default(r.Logger).Info("ruleset initialized", slog.Int("revision", rs.Revision), slog.Int("rules", len(rs.Rules)), slog.Bool("seeded", !ok && seed != ""))
	return nil
}

// Get returns the current set.
func (r *Rules) Get() *domain.Ruleset { return r.Cell.Load() }

// GetRule returns one rule or API_NOT_FOUND.
func (r *Rules) GetRule(id string) (domain.Rule, error) {
	if rule, ok := r.Cell.Load().Find(id); ok {
		return rule, nil
	}
	return domain.Rule{}, codes.New(codes.APINotFound, "rule %s not found", id)
}

// Replace substitutes the whole set.
func (r *Rules) Replace(ifMatch string, raw []byte) (*domain.Ruleset, error) {
	return r.change(ifMatch, nil, func(_ *domain.Ruleset) (*domain.Ruleset, []string, error) {
		next, cerr := domain.ParseRuleset(raw)
		if cerr != nil {
			return nil, nil, cerr
		}
		ids := []string{}
		for _, x := range next.Rules {
			ids = append(ids, x.ID)
		}
		return next, ids, nil
	})
}

// Add appends one rule.
func (r *Rules) Add(ifMatch string, raw []byte) (*domain.Ruleset, error) {
	return r.change(ifMatch, raw, func(cur *domain.Ruleset) (*domain.Ruleset, []string, error) {
		rule, _ := domain.ParseRule(raw)
		return cur.With(rule), []string{rule.ID}, nil
	})
}

// Update replaces the rule id.
func (r *Rules) Update(ifMatch, id string, raw []byte) (*domain.Ruleset, error) {
	return r.change(ifMatch, raw, func(cur *domain.Ruleset) (*domain.Ruleset, []string, error) {
		rule, _ := domain.ParseRule(raw)
		next, ok := cur.Replacing(id, rule)
		if !ok {
			return nil, nil, codes.New(codes.APINotFound, "rule %s not found", id)
		}
		return next, []string{id, rule.ID}, nil
	})
}

// Delete removes the rule id.
func (r *Rules) Delete(ifMatch, id string) (*domain.Ruleset, error) {
	return r.change(ifMatch, nil, func(cur *domain.Ruleset) (*domain.Ruleset, []string, error) {
		next, ok := cur.Without(id)
		if !ok {
			return nil, nil, codes.New(codes.APINotFound, "rule %s not found", id)
		}
		return next, []string{id}, nil
	})
}

// change checks If-Match, validates one rule if given, builds the next set, checks RULE-1..5, persists and swaps.
func (r *Rules) change(ifMatch string, rule []byte, build func(*domain.Ruleset) (*domain.Ruleset, []string, error)) (*domain.Ruleset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.Cell.Load()
	if ifMatch == "" {
		return nil, codes.New(codes.APIPreconditionRequired, "rule changes require If-Match")
	}
	if ifMatch != ETag(cur.Revision) {
		return nil, codes.New(codes.APIPreconditionFailed, "If-Match %s does not match revision %d", ifMatch, cur.Revision)
	}
	if rule != nil {
		if _, cerr := domain.ParseRule(rule); cerr != nil {
			return nil, cerr
		}
	}
	next, ids, err := build(cur)
	if err != nil {
		return nil, err
	}
	next.Revision = cur.Revision + 1
	if cerr := next.Check(r.HasManagement); cerr != nil {
		return nil, cerr
	}
	if err := r.Store.Save(next); err != nil {
		return nil, err
	}
	r.Cell.Store(next)
	n := notice(EvRules, r.Namespace, r.Clock.Now(), "", "rule set changed")
	n.Revision, n.Rules = next.Revision, ids
	r.Events.Emit(n)
	logx.Default(r.Logger).Info("ruleset updated", slog.Int("revision", next.Revision), slog.Int("changed_rules", len(ids)))
	return next, nil
}
