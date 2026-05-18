package domain

import (
	"time"

	"gorm.io/gorm"
)

type Timestamp struct {
	CreatedAt time.Time `gorm:"column:created_at;not null;type:datetime;default:current_timestamp"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;type:datetime;default:current_timestamp"`
}

type TimestampWithDeletedAt struct {
	Timestamp
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:datetime;index"`
}
