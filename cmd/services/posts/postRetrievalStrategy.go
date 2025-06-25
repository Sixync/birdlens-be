package services

import (
	"context"
	"time"

	"github.com/sixync/birdlens-be/internal/store"
)

type PostRetriever interface {
	RetrievePosts(ctx context.Context, userId int64, limit, offset int) (*store.PaginatedList[*store.Post], error)
}

type trendingPostRetriever struct {
	store *store.Storage
}

func NewTrendingPostRetriever(store *store.Storage) *trendingPostRetriever {
	return &trendingPostRetriever{
		store: store,
	}
}

// Trending posts within the last 7 days
// Trending posts are posts that have received the most likes in the last 7 days
func (r *trendingPostRetriever) RetrievePosts(ctx context.Context, userId int64, limit, offset int) (*store.PaginatedList[*store.Post], error) {
	duration := time.Now().AddDate(0, 0, -7) // 7 days ago
	posts, err := r.store.Posts.GetTrendingPosts(ctx, duration, limit, offset)
	if err != nil {
		return nil, err
	}

	return posts, nil
}

type allPostRetriever struct {
	store *store.Storage
}

func NewAllPostRetriever(store *store.Storage) *allPostRetriever {
	return &allPostRetriever{
		store: store,
	}
}

func (r *allPostRetriever) RetrievePosts(ctx context.Context, userId int64, limit, offset int) (*store.PaginatedList[*store.Post], error) {
	posts, err := r.store.Posts.GetAll(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return posts, nil
}

type followerPostsRetriever struct {
	store *store.Storage
}

func NewFollowerPostsRetriever(store *store.Storage) *followerPostsRetriever {
	return &followerPostsRetriever{
		store: store,
	}
}

func (r *followerPostsRetriever) RetrievePosts(ctx context.Context, userId int64, limit, offset int) (*store.PaginatedList[*store.Post], error) {
	posts, err := r.store.Posts.GetFollowerPosts(ctx, userId, limit, offset)
	if err != nil {
		return nil, err
	}
	return posts, nil
}
