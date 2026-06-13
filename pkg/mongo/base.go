package mongo

import "time"

// BaseModel is the base model for all Mongo documents
type BaseModel struct {
	ID        string    `bson:"_id" json:"id"`
	CreatedBy string    `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy string    `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
	Deleted   bool      `bson:"deleted" json:"deleted"`
}

func (bm *BaseModel) UpdateTimestamps() {
	now := time.Now()
	if bm.CreatedAt.IsZero() {
		bm.CreatedAt = now
	}
	bm.UpdatedAt = now
}
