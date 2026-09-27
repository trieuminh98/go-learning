package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"go-learning/internal/core"
	"go-learning/internal/db"
	generated "go-learning/internal/db/generated"
	"go-learning/internal/repository"
)

var firstNames = []string{
	"Alice", "Bob", "Charlie", "David", "Emma",
	"Fiona", "George", "Hannah", "Ian", "Julia",
	"Kevin", "Laura", "Michael", "Nina", "Oliver",
	"Paula", "Quinn", "Rachel", "Sam", "Tina",
}

var lastNames = []string{
	"Nguyen", "Tran", "Le", "Pham", "Hoang",
	"Phan", "Vu", "Dang", "Bui", "Do",
	"Smith", "Johnson", "Williams", "Brown", "Jones",
	"Garcia", "Miller", "Davis", "Taylor", "Anderson",
}

var postTitles = []string{
	"Getting Started with Go and Clean Architecture",
	"Optimistic Concurrency Control Explained",
	"Designing High-Performance PostgreSQL Queries",
	"Database Query Timeouts and Context Deadlines",
	"Mastering SQL with sqlc and pgx in Go",
	"Building Robust REST APIs with Go Standard Library",
	"Dependency Injection Without Frameworks in Go",
	"Understanding Connection Pools in Distributed Systems",
	"Graceful Shutdown and Request Lifecycles",
	"Pagination Strategies for High-Volume Feeds",
}

var postTopics = []string{
	"Here is an in-depth breakdown of practical patterns we learned while building concurrent backend systems in Go.",
	"When designing relational models, understanding query isolation and foreign key cascades is essential.",
	"A walkthrough of context propagation, timeout budgets, and why context.Background() should never enter repositories.",
	"Exploring partial updates with PATCH, field presence, and pointer semantics to distinguish nil from empty values.",
	"Real-world trade-offs between optimistic locking with version columns versus in-memory mutexes.",
}

var tagPool = [][]string{
	{"golang", "backend"},
	{"database", "postgresql", "sqlc"},
	{"architecture", "clean-code"},
	{"concurrency", "performance"},
	{"api", "rest", "http"},
}

var commentTemplates = []string{
	"Great explanation! Really helped clarify the context budget concept.",
	"Thanks for sharing this, the diagram made it very intuitive.",
	"How does this handle high write contention when multiple clients retry?",
	"Solid architectural advice. Keeping repositories decoupled from HTTP is crucial.",
	"Looking forward to the next lesson on follower feeds and pagination!",
	"One question: what would be the recommended timeout for batch processing?",
}

func main() {
	dbURL := flag.String("db-url", os.Getenv("DB_URL"), "PostgreSQL database connection URL")
	numUsers := flag.Int("users", 100, "Number of users to seed")
	numPosts := flag.Int("posts", 200, "Number of posts to seed")
	numComments := flag.Int("comments", 300, "Number of comments to seed")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = "postgres://postgres:password@localhost:5433/golearning?sslmode=disable"
	}

	log.Printf("Connecting to database for seeding...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(
		ctx,
		*dbURL,
		5,
		1,
		15*time.Minute,
		1*time.Hour,
	)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := generated.New(pool)
	storage := repository.NewPostgresStorage(queries)

	seedCtx, seedCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer seedCancel()

	// 1. Seed Users
	log.Printf("Seeding %d users...", *numUsers)
	userIDs := make([]int64, 0, *numUsers)
	for i := 0; i < *numUsers; i++ {
		fn := firstNames[i%len(firstNames)]
		ln := fmt.Sprintf("%s%d", lastNames[(i/len(firstNames))%len(lastNames)], i+1)

		createdUser, err := storage.Users.Create(seedCtx, core.User{
			FirstName: fn,
			LastName:  ln,
		})
		if err != nil {
			log.Fatalf("failed to seed user %d: %v", i+1, err)
		}

		id, err := strconv.ParseInt(createdUser.ID, 10, 64)
		if err != nil {
			log.Fatalf("failed to parse user ID %q: %v", createdUser.ID, err)
		}
		userIDs = append(userIDs, id)
	}
	log.Printf("Successfully created %d users.", len(userIDs))

	// 2. Seed Posts (foreign key: user_id -> users.id)
	log.Printf("Seeding %d posts...", *numPosts)
	postIDs := make([]int64, 0, *numPosts)
	for i := 0; i < *numPosts; i++ {
		authorUserID := userIDs[i%len(userIDs)]
		title := fmt.Sprintf("%s #%d", postTitles[i%len(postTitles)], i+1)
		content := fmt.Sprintf("%s This is post #%d discussing key architectural decisions.", postTopics[i%len(postTopics)], i+1)
		tags := tagPool[i%len(tagPool)]

		createdPost, err := storage.Posts.Create(seedCtx, core.Post{
			UserID:  authorUserID,
			Title:   title,
			Content: content,
			Tags:    tags,
		})
		if err != nil {
			log.Fatalf("failed to seed post %d: %v", i+1, err)
		}
		postIDs = append(postIDs, createdPost.ID)
	}
	log.Printf("Successfully created %d posts.", len(postIDs))

	// 3. Seed Comments (foreign keys: post_id -> posts.id, user_id -> users.id)
	log.Printf("Seeding %d comments...", *numComments)
	for i := 0; i < *numComments; i++ {
		targetPostID := postIDs[i%len(postIDs)]
		commenterUserID := userIDs[(i*7+3)%len(userIDs)]
		commentText := commentTemplates[i%len(commentTemplates)]

		_, err := storage.Comments.Create(seedCtx, core.Comment{
			PostID:  targetPostID,
			UserID:  commenterUserID,
			Content: fmt.Sprintf("[%d] %s", i+1, commentText),
		})
		if err != nil {
			log.Fatalf("failed to seed comment %d: %v", i+1, err)
		}
	}
	log.Printf("Successfully created %d comments.", *numComments)

	log.Printf("Database seeding completed successfully! Summary: %d users, %d posts, %d comments.",
		len(userIDs), len(postIDs), *numComments)
}
