package models

import (
	"fmt"
	"time"
)

type Resource struct {
	Name         string
	Env          string
	LastActivity time.Time
}

func ResourceKey(name, env string) string {
	return fmt.Sprintf("%s_%s", env, name)
}

func (r *Resource) Key() string {
	return ResourceKey(r.Name, r.Env)
}

func (r *Resource) String() string {
	if r.Env != "" {
		return fmt.Sprintf("%s|%s", r.Env, r.Name)
	}
	return r.Name
}
