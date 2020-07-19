package data

import (
	"sort"
	"sync"
	"time"

	"github.com/ameliagapin/reservebot/err"
	"github.com/ameliagapin/reservebot/models"
)

type DataManager interface {
	GetAllUsersInQueues() []*models.User
	GetPosition(u *models.User, name string, env string) (int, error)
	GetQueueForResource(name string, env string) *models.Queue
	GetQueues() []*models.Reservation
	GetQueuesForEnv(env string) map[string]*models.Queue
	GetReservation(u *models.User, name string, env string) *models.Reservation
	GetReservationForResource(name string, env string) (*models.Reservation, error)
	GetResource(name string, env string, create bool) *models.Resource
	GetResources() []*models.Resource
	GetResourcesForEnv(env string) []*models.Resource
	Remove(u *models.User, name string, env string) error
	RemoveEnv(name string, env string) error
	RemoveResource(name string, env string) error
	Reserve(u *models.User, name string, env string) error
}

type Memory struct {
	Reservations []*models.Reservation
	Resources    map[string]*models.Resource

	lock sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{
		Reservations: []*models.Reservation{},
	}
}

func (m *Memory) Reserve(u *models.User, name, env string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	r := m.GetResource(name, env, true)

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
	m.lock.Lock()
	defer m.lock.Unlock()

	r := m.GetResource(name, env, false)
	if r == nil {
		return nil
	}

	for _, res := range m.Reservations {
		if res.User.ID == u.ID {
			if res.Resource.Key() == r.Key() {
				return res
			}
		}
	}
	return nil
}

func (m *Memory) Remove(u *models.User, name, env string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// minor optimization: if the resource doens't exist, there's no need to loop through all reservations
	r := m.GetResource(name, env, false)
	if r == nil {
		return err.ResourceDoesNotExist
	}

	idx := -1
	for i, res := range m.Reservations {
		if res.User.ID == u.ID {
			if res.Resource.Key() == r.Key() {
				idx = i
				break
			}
			// TODD: update new person holding resouce's reservation time

		}
	}
	if idx == -1 {
		return err.NotInQueue
	}

	m.Reservations = append(m.Reservations[:idx], m.Reservations[idx+1:]...)
	return nil
}

func (m *Memory) GetPosition(u *models.User, name, env string) (int, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	r := m.GetResource(name, env, false)
	if r == nil {
		return 0, err.ResourceDoesNotExist
	}

	pos := 0

	for _, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			pos++
			if res.User.ID == u.ID {
				break
			}
		}
	}
	return pos, nil
}

func (m *Memory) GetResource(name, env string, create bool) *models.Resource {
	m.lock.Lock()
	defer m.lock.Unlock()

	key := models.ResourceKey(name, env)
	r, ok := m.Resources[key]
	if !ok {
		if !create {
			return nil
		}
		r = &models.Resource{
			Name: name,
			Env:  env,
		}
		m.Resources[r.Key()] = r
	}
	return r
}

func (m *Memory) RemoveResource(name, env string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	r := m.GetResource(name, env, false)
	if r == nil {
		return err.ResourceDoesNotExist
	}

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

func (m *Memory) GetQueues() []*models.Reservation {
	return m.Reservations
}

func (m *Memory) GetQueueForResource(name, env string) *models.Queue {
	m.lock.Lock()
	defer m.lock.Unlock()

	ret := &models.Queue{}

	// minor optimization
	r := m.GetResource(name, env, false)
	if r == nil {
		return ret
	}

	for _, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			ret.Reservations = append(ret.Reservations, res)
		}
	}

	return ret
}

func (m *Memory) GetReservationForResource(name, env string) (*models.Reservation, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// minor optimization
	r := m.GetResource(name, env, false)
	if r == nil {
		return nil, err.ResourceDoesNotExist
	}

	for _, res := range m.Reservations {
		if res.Resource.Key() == r.Key() {
			return res, nil
		}
	}

	return nil, nil
}

func (m *Memory) GetQueuesForEnv(env string) map[string]*models.Queue {
	m.lock.Lock()
	defer m.lock.Unlock()

	ret := make(map[string]*models.Queue)

	resources := m.GetResourcesForEnv(env)
	for _, r := range resources {
		q := m.GetQueueForResource(r.Name, r.Env)
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
