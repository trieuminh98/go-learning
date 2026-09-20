package memory_test

import (
	"context"
	"testing"

	"go-learning/internal/core"
	"go-learning/internal/repository/memory"
)

func TestUserRepository(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserRepository()

	t.Run("initially empty", func(t *testing.T) {
		users, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(users) != 0 {
			t.Fatalf("expected 0 users, got %d", len(users))
		}
	})

	t.Run("create user assigns id and persists", func(t *testing.T) {
		u := core.User{FirstName: "John", LastName: "Doe"}
		created, err := repo.Create(ctx, u)
		if err != nil {
			t.Fatalf("unexpected error creating user: %v", err)
		}
		if created.ID == "" {
			t.Fatal("expected assigned ID, got empty")
		}

		users, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("unexpected error listing users: %v", err)
		}
		if len(users) != 1 {
			t.Fatalf("expected 1 user, got %d", len(users))
		}
		if users[0].FirstName != "John" || users[0].LastName != "Doe" {
			t.Fatalf("unexpected user content: %+v", users[0])
		}
	})

	t.Run("duplicate user rejected", func(t *testing.T) {
		u := core.User{FirstName: "john", LastName: "doe"}
		_, err := repo.Create(ctx, u)
		if err != core.ErrUserAlreadyExists {
			t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
		}
	})
}

func TestTruckRepository(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewTruckRepository()

	t.Run("seeded trucks present", func(t *testing.T) {
		trucks, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(trucks) < 2 {
			t.Fatalf("expected at least 2 seeded trucks, got %d", len(trucks))
		}
	})

	t.Run("get by id existing and non-existing", func(t *testing.T) {
		truck, err := repo.GetByID(ctx, "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if truck.ID != "1" {
			t.Fatalf("expected truck 1, got %s", truck.ID)
		}

		_, err = repo.GetByID(ctx, "non-existent")
		if err != core.ErrTruckNotFound {
			t.Fatalf("expected ErrTruckNotFound, got %v", err)
		}
	})

	t.Run("create new truck and duplicate check", func(t *testing.T) {
		tNew := core.Truck{ID: "3", Capacity: 7000, CurrentLoad: 1000}
		if err := repo.Create(ctx, tNew); err != nil {
			t.Fatalf("unexpected error creating truck: %v", err)
		}

		if err := repo.Create(ctx, tNew); err != core.ErrDuplicateTruckID {
			t.Fatalf("expected ErrDuplicateTruckID, got %v", err)
		}
	})
}
