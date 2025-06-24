package services

import (
	"context"

	"github.com/sixync/birdlens-be/internal/store"
)

type postRetriever interface {
	RetrievePosts(ctx context.Context, userId int64, limit, offset int) (*store.PaginatedList[*store.Post], error)
}

type trendingPostRetriever struct {
	store *store.Storage
}

// // Trending posts within the last 7 days
// func (r *trendingPostRetriever) RetrievePosts(ctx context.Context, userId int64, limit, offset int) (*store.PaginatedList[*store.Post], error) {
// 	duration := time.Now().AddDate(0, 0, -7) // 7 days ago
// 	posts, err := r.store.Posts.GetTrendingPosts(ctx, duration, limit, offset)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return posts, nil
// }
//
// type allPostRetriever struct {
// 	store *store.Storage
// }
//
// func (r *allPostRetriever) RetrievePosts(ctx context.Context, userId int64, limit, offset int) (*store.PaginatedList[*store.Post], error) {
// 	posts, err := r.store.Posts.GetAll(ctx, limit, offset)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return posts, nil
// }
