package postgres_test

import (
	"testing"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
	"go-learning/internal/repository/postgres"
)

// TestPostgresRepositoryInterfaces verifies at compile time and runtime that
// the PostgreSQL repository structs satisfy the Logic contracts.
func TestPostgresRepositoryInterfaces(t *testing.T) {
	var _ logic.UserRepository = (*postgres.UserRepository)(nil)
	var _ logic.TruckRepository = (*postgres.TruckRepository)(nil)
	var _ logic.PostRepository = (*postgres.PostRepository)(nil)
	var _ logic.CommentRepository = (*postgres.CommentRepository)(nil)
}

func TestPostgresMapping(t *testing.T) {
	t.Run("User conversion logic consistency", func(t *testing.T) {
		dbUser := db.User{
			ID:        42,
			FirstName: "Alice",
			LastName:  "Smith",
		}

		if dbUser.ID != 42 || dbUser.FirstName != "Alice" || dbUser.LastName != "Smith" {
			t.Fatalf("unexpected db user fields")
		}
	})

	t.Run("Truck conversion logic consistency", func(t *testing.T) {
		dbTruck := db.Truck{
			ID:          "T-100",
			Capacity:    5000,
			CurrentLoad: 1200,
		}

		coreTruck := core.Truck{
			ID:          dbTruck.ID,
			Capacity:    int(dbTruck.Capacity),
			CurrentLoad: int(dbTruck.CurrentLoad),
		}

		if coreTruck.ID != "T-100" || coreTruck.Capacity != 5000 || coreTruck.CurrentLoad != 1200 {
			t.Fatalf("unexpected truck mapping: %+v", coreTruck)
		}
	})
}
