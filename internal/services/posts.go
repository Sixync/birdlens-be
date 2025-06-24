package services

import (
	"context"
	"time"

	"github.com/sixync/birdlens-be/internal/store"
)

type PostRetrievalStrategy interface {
	RetrievePosts(ctx context.Context, userId int64, duration time.Time, limit, offset int) (*store.PaginatedList[*store.Post], error)
}

type TrendingPostsStrategy struct {
	Store *store.Storage
}

// func (s *TrendingPostsStrategy) RetrievePosts(ctx context.Context, userId int64, duration time.Time, limit, offset int) (*store.PaginatedList[*store.Post], error) {
// 	// trending posts are posts that have mosts likes in 7 days
// 	posts, err := s.Store.Posts.GetTrendingPosts(ctx, duration, limit, offset)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return posts, nil
// }
