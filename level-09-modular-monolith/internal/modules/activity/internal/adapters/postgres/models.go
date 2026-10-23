package postgres

import "time"

// activityModel is the row shape of the activity.activities table.
type activityModel struct {
	ID        string
	TodoID    string
	Action    string
	CreatedAt time.Time `gorm:"autoCreateTime:false"`
}

func (activityModel) TableName() string { return "activity.activities" }
