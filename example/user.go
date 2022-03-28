package example

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

//{"collection_name":"user", "indexes": [{"keys":{"name":1, "shoe_size": 1},"options":{"unique": true}},{"keys":{"name":1}},{"keys":{"name":1, "age": 1}},{"keys":{"profile":1}}]}
type User struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	ShoeSize  int                `bson:"shoe_size"`
	Age       int                `bson:"age"`
	CreatedAt time.Time          `bson:"created_at"`
	Profile   Profile            `bson:"profile"`
}

type Profile struct {
	FavoriteColor string `bson:"favorite_color"`
}

// TODO: check if ID exist, or check for the primary key
// TODO: check if createdAt exist
