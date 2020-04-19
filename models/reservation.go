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
		return errors.New(fmt.Sprintf("Resource %s does not currently exist", resource))
	}

	for i, u := range queue {
		if u.User.ID == user.ID {
			r.Reservations[resource] = append(queue[:i], queue[i+1:]...)
		}
	}

	return errors.New(fmt.Sprintf("<@%s> is not in line for %s", user.ID, resource))
}

func (r *Reservations) GetPosition(resource string, user *User) (int, error) {
	queue, exists := r.Reservations[resource]
	if !exists {
		return 0, errors.New(fmt.Sprintf("Resource %s does not currently exist", resource))
	}

	for i, u := range queue {
		if u.User.ID == user.ID {
			return i + 1, nil
		}
	}

	return 0, nil
}
