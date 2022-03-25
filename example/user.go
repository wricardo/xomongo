package example

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

//{"collection_name":"user", "indexes": [{"name":1}]}
type User struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	CreatedAt time.Time          `bson:"created_at"`
	Profile   Profile            `bson:"profile"`
}

type Profile struct {
	FavoriteColor string `bson:"favorite_color"`
}

// TODO: check if ID exist, or check for the primary key
// TODO: check if createdAt exist
