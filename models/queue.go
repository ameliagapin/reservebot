package models

type Queue struct {
	Resource     *Resource
	Reservations []*Reservation
}
