package mongo

import "time"

// activitiesCollection is prefixed with the module's name, as the Postgres table is
// qualified with its schema: "activity.activities" in both. Prepare creates it.
const activitiesCollection = "activity.activities"

// activityDoc is the BSON shape of an activity.
type activityDoc struct {
	ID        string    `bson:"_id"`
	TodoID    string    `bson:"todo_id"`
	Action    string    `bson:"action"`
	CreatedAt time.Time `bson:"created_at"`
}
