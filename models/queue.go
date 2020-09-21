package models

type Queue struct {
	Resource     *Resource
	Reservations []*Reservation
}

func (q *Queue) HasReservations() bool {
	return len(q.Reservations) > 0
}
