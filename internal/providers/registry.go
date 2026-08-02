package providers

import (
	"fmt"
	"sort"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

type Registry struct {
	byID map[domain.ProviderID]domain.Provider
}

func NewRegistry(items ...domain.Provider) *Registry {
	byID := make(map[domain.ProviderID]domain.Provider, len(items))
	for _, item := range items {
		if _, exists := byID[item.ID()]; exists {
			panic(fmt.Sprintf("duplicate provider registration: %s", item.ID()))
		}
		byID[item.ID()] = item
	}
	return &Registry{byID: byID}
}

func (r *Registry) Get(id domain.ProviderID) (domain.Provider, bool) {
	provider, ok := r.byID[id]
	return provider, ok
}

func (r *Registry) All() []domain.Provider {
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	result := make([]domain.Provider, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.byID[domain.ProviderID(id)])
	}
	return result
}
