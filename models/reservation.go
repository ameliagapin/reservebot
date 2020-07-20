package models

import (
	"time"
)

type Reservation struct {
	User     *User
	Resource *Resource
	Time     time.Time
}
