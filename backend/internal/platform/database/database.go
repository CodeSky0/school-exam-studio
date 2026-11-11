package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	entgo "entgo.io/ent"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/paper"
	"school-exam-studio/backend/ent/paperquestion"
	"school-exam-studio/backend/ent/question"
	"school-exam-studio/backend/ent/school"
	"school-exam-studio/backend/ent/user"
)

func Open(ctx context.Context, databaseURL string) (*ent.Client, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	client.Intercept(entgo.InterceptFunc(func(next entgo.Querier) entgo.Querier {
		return entgo.QuerierFunc(func(ctx context.Context, query entgo.Query) (entgo.Value, error) {
			switch q := query.(type) {
			case *ent.SchoolQuery:
				q.Where(school.DeletedAtIsNil())
			case *ent.UserQuery:
				q.Where(user.DeletedAtIsNil())
			case *ent.QuestionQuery:
				q.Where(question.DeletedAtIsNil())
			case *ent.PaperQuery:
				q.Where(paper.DeletedAtIsNil())
			case *ent.PaperQuestionQuery:
				q.Where(paperquestion.DeletedAtIsNil())
			}
			return next.Query(ctx, query)
		})
	}))
	return client, nil
}
