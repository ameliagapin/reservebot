package models

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type Reservation struct {
	User *User
	Time time.Time
}

type Reservations struct {
	Reservations map[string][]*Reservation

	lock sync.Mutex
}

func NewReservations() *Reservations {
	return &Reservations{
		Reservations: map[string][]*Reservation{},
	}
}

func (r *Reservations) Add(resource string, user *User) error {
	res := &Reservation{
		User: user,
		Time: time.Now(),
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	queue, exists := r.Reservations[resource]
	if !exists {
		r.Reservations[resource] = []*Reservation{res}
		return nil
	}

	for _, u := range queue {
		if u.User.ID == user.ID {
			//return errors.New(fmt.Sprintf("<@%s> is already in line for %s", user.ID, resource))
			return nil
		}
	}

	r.Reservations[resource] = append(queue, res)

	return nil
}

func (r *Reservations) Remove(resource string, user *User) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	queue, exists := r.Reservations[resource]
	if !exists {
		return errors.New(fmt.Sprintf("Resource `%s` does not currently exist", resource))
	}

	for i, u := range queue {
		if u.User.ID == user.ID {
			r.Reservations[resource] = append(queue[:i], queue[i+1:]...)
			return nil
		}
	}

	return errors.New(fmt.Sprintf("<@%s> is not in line for `%s`", user.ID, resource))
}

func (r *Reservations) Clear(resource string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	_, exists := r.Reservations[resource]
	if !exists {
		return errors.New(fmt.Sprintf("Resource `%s` does not currently exist", resource))
	}

	r.Reservations[resource] = []*Reservation{}

	return nil
}
func (r *Reservations) GetPosition(resource string, user *User) (int, error) {
	queue, exists := r.Reservations[resource]
	if !exists {
		return 0, errors.New(fmt.Sprintf("Resource `%s` does not currently exist", resource))
	}

	for i, u := range queue {
		if u.User.ID == user.ID {
			return i + 1, nil
		}
	}

	return 0, nil
}

func (r *Reservations) GetQueueForResource(resource string) ([]*Reservation, error) {
	queue, exists := r.Reservations[resource]
	if !exists {
		return []*Reservation{}, nil
	}

	return queue, nil
}

func (r *Reservations) GetReservationForResource(resource string) (*Reservation, error) {
	queue, err := r.GetQueueForResource(resource)
	if err != nil {
		return nil, err
	}

	if len(queue) == 0 {
		return nil, nil
	}

	return queue[0], nil
}

func (r *Reservations) GetResources() []string {
	ret := []string{}

	for k, _ := range r.Reservations {
		ret = append(ret, k)
	}
	return ret
}
