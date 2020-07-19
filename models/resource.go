package models

import "fmt"

type Resource struct {
	Name string
	Env  string
}

func ResourceKey(name, env string) string {
	return fmt.Sprintf("%s_%s", env, name)
}

func (r *Resource) Key() string {
	return ResourceKey(r.Env, r.Name)
}

func (r *Resource) String() string {
	if r.Env != "" {
		return fmt.Sprintf("%s|%s", r.Env, r.Name)
	}
	return r.Name
}
