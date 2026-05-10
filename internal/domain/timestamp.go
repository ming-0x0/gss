package domain

import "time"

type Timestamp struct {
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type TimestampWithDeletedAt struct {
	Timestamp
	DeletedAt *time.Time `bun:"deleted_at,nullzero"`
}
