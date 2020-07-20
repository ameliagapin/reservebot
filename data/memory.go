package data

import (
	"sort"
	"sync"
	"time"

	"github.com/ameliagapin/reservebot/err"
	"github.com/ameliagapin/reservebot/models"
)

type Memory struct {
	Reservations []*models.Reservation
	Resources    map[string]*models.Resource

	lock sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{
		Reservations: []*models.Reservation{},
		Resources:    map[string]*models.Resource{},
	}
}

func (m *Memory) Reserve(u *models.User, name, env string) error {
	r := m.GetResource(name, env, true)

	m.lock.Lock()
	defer m.lock.Unlock()

	// check for existing reservation
	for _, res := range m.Reservations {
		if res.User.ID == u.ID {
			if res.Resource.Key() == r.Key() {
				return err.AlreadyInQueue
			}
		}
	}

	res := &models.Reservation{
		User:     u,
		Resource: r,
		Time:     time.Now(),
	}

	m.Reservations = append(m.Reservations, res)

	return nil
}

func (m *Memory) GetReservation(u *models.User, name, env string) *models.Reservation {
	r := m.GetResource(name, env, false)
	if r == nil {
		return nil
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, res := range m.Reservations {
		if res.User.ID == u.ID {
			if res.Resource.Key() == r.Key() {
				return res
			}
		}
	}
	return nil
}

// Remove removes a user from a resource's queue.
// If the removal advances the queue, the new resource holder's reservation will have the time updated
func (m *Memory) Remove(u *models.User, name, env string) error {
	// minor optimization: if the resource doesn't exist, there's no need to loop through all reservations
	r := m.GetResource(name, env, false)
	if r == nil {
		return err.ResourceDoesNotExist
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	idx := -1
	pos := 0
	for i, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			pos++
			if res.User.ID == u.ID {
				idx = i
				continue
			}
		}
	}
	if idx == -1 {
		return err.NotInQueue
	}

	m.Reservations = append(m.Reservations[:idx], m.Reservations[idx+1:]...)

	// if the user was in pos=1, then removal would move new user into pos=1. This should update the time on their res
	if pos == 1 {
		for _, res := range m.Reservations {
			if res.Resource.Key() == r.Key() {
				res.Time = time.Now()
			}
		}

	}

	return nil
}

func (m *Memory) GetPosition(u *models.User, name, env string) (int, error) {
	r := m.GetResource(name, env, false)
	if r == nil {
		return 0, err.ResourceDoesNotExist
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	pos := 0
	inQueue := false

	for _, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			// increment pos first because want to return zero-based index
			pos++
			if res.User.ID == u.ID {
				inQueue = true
				break
			}
		}
	}
	if !inQueue {
		return 0, err.NotInQueue
	}

	return pos, nil
}

func (m *Memory) GetResource(name, env string, create bool) *models.Resource {
	m.lock.Lock()
	defer m.lock.Unlock()

	key := models.ResourceKey(name, env)
	r, ok := m.Resources[key]
	if !ok {
		if create {
			r = &models.Resource{
				Name: name,
				Env:  env,
			}
			m.Resources[r.Key()] = r
		}
	}
	return r
}

func (m *Memory) RemoveResource(name, env string) error {
	r := m.GetResource(name, env, false)
	if r == nil {
		return err.ResourceDoesNotExist
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	for idx, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			m.Reservations = append(m.Reservations[:idx], m.Reservations[idx+1:]...)
		}
	}

	delete(m.Resources, r.Key())

	return nil
}

func (m *Memory) RemoveEnv(name, env string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	exists := false
	for idx, res := range m.Reservations {
		if res.Resource.Env == env {
			m.Reservations = append(m.Reservations[:idx], m.Reservations[idx+1:]...)
			exists = true
		}
	}

	for k, res := range m.Resources {
		if res.Env == env {
			delete(m.Resources, k)
			exists = true
		}
	}

	if !exists {
		return err.EnvDoesNotExist
	}

	return nil
}

func (m *Memory) GetResources() []*models.Resource {
	m.lock.Lock()
	defer m.lock.Unlock()

	keys := []string{}
	for k, _ := range m.Resources {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ret := []*models.Resource{}
	for _, k := range keys {
		ret = append(ret, m.Resources[k])
	}

	return ret
}

// Does not implement lock
func (m *Memory) GetQueues() []*models.Queue {
	ret := []*models.Queue{}

	resources := m.GetResources()
	for _, r := range resources {
		q, _ := m.GetQueueForResource(r.Name, r.Env)
		ret = append(ret, q)
	}

	return ret
}

func (m *Memory) GetQueueForResource(name, env string) (*models.Queue, error) {
	// minor optimization
	r := m.GetResource(name, env, false)
	if r == nil {
		return nil, err.ResourceDoesNotExist
	}

	ret := &models.Queue{
		Resource: r,
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			ret.Reservations = append(ret.Reservations, res)
		}
	}

	return ret, nil
}

func (m *Memory) GetReservationForResource(name, env string) (*models.Reservation, error) {
	// minor optimization
	r := m.GetResource(name, env, false)
	if r == nil {
		return nil, err.ResourceDoesNotExist
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			return res, nil
		}
	}

	return nil, nil
}

// Does not implement lock
func (m *Memory) GetQueuesForEnv(env string) map[string]*models.Queue {
	ret := make(map[string]*models.Queue)

	resources := m.GetResourcesForEnv(env)
	for _, r := range resources {
		q, _ := m.GetQueueForResource(r.Name, r.Env)
		ret[r.Name] = q
	}

	return ret
}

func (m *Memory) GetResourcesForEnv(env string) []*models.Resource {
	m.lock.Lock()
	defer m.lock.Unlock()

	keys := []string{}
	for k, r := range m.Resources {
		if r.Env == env {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	ret := []*models.Resource{}
	for _, k := range keys {
		ret = append(ret, m.Resources[k])
	}
	return ret
}

func (m *Memory) GetAllUsersInQueues() []*models.User {
	m.lock.Lock()
	defer m.lock.Unlock()

	all := map[string]*models.User{}

	for _, r := range m.Reservations {
		all[r.User.ID] = r.User
	}

	ret := []*models.User{}
	for _, u := range all {
		ret = append(ret, u)
	}

	return ret
}

func (m *Memory) ClearQueueForResource(name, env string) error {
	// minor optimization
	r := m.GetResource(name, env, false)
	if r == nil {
		return err.ResourceDoesNotExist
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	filtered := []*models.Reservation{}
	for _, res := range m.Reservations {
		if res.Resource.Key() != r.Key() {
			filtered = append(filtered, res)
		}
	}
	m.Reservations = filtered

	return nil
}
