// path: birdlens-be/internal/store/store.go
package store

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jmoiron/sqlx"
)

var QueryTimeoutDuration = 5 * time.Second

type Storage struct {
	Users interface {
		Create(context.Context, *User) error
		GetById(ctx context.Context, userId int64) (*User, error)
		Update(ctx context.Context, user *User) error
		Delete(ctx context.Context, userId int64) error
		GetByEmail(ctx context.Context, email string) (*User, error)
		GetByUsername(ctx context.Context, username string) (*User, error)
		UsernameExists(ctx context.Context, username string) (bool, error)
		EmailExists(ctx context.Context, username string) (bool, error)
		GetByFirebaseUID(ctx context.Context, firebaseUID string) (*User, error)
		AddEmailVerificationToken(ctx context.Context, userId int64, token string, expiresAt time.Time) error
		GetEmailVerificationToken(ctx context.Context, userId int64) (token string, expiresAt time.Time, err error)
		VerifyUserEmail(ctx context.Context, userId int64) error
		// Logic: The old Stripe-specific method is now removed from the interface definition.
		// UpdateUserSubscription(ctx context.Context, userID int64, subscriptionID int64, stripeCustomerID, stripeSubscriptionID, stripePriceID, stripeStatus string, periodEnd time.Time) error
		GetSubscriptionByName(ctx context.Context, name string) (*Subscription, error)
		AddResetPasswordToken(ctx context.Context, email string, token string, expiresAt time.Time) error
		GetUserByResetPasswordToken(ctx context.Context, token string) (*User, error)
		GrantSubscriptionForOrder(ctx context.Context, userID int64, subscriptionID int64) error
	}
	Posts interface {
		Create(context.Context, *Post) error
		GetById(ctx context.Context, postId int64) (*Post, error)
		Update(ctx context.Context, post *Post) error
		Delete(ctx context.Context, postId int64) error
		GetAll(ctx context.Context, limit, offset int) (*PaginatedList[*Post], error)
		GetLikeCounts(ctx context.Context, postId int64) (int, error)
		GetCommentCounts(ctx context.Context, postId int64) (int, error)
		GetMediaUrlsById(ctx context.Context, postId int64) ([]string, error)
		UserLiked(ctx context.Context, userId, postId int64) (bool, error)
		AddUserReaction(ctx context.Context, userId, postId int64, reactionType string) error
		AddMediaUrl(ctx context.Context, postId int64, mediaUrls string) error
		GetTrendingPosts(ctx context.Context, duration time.Time, limit, offset int) (*PaginatedList[*Post], error)
		GetFollowerPosts(ctx context.Context, userId int64, limit, offset int) (*PaginatedList[*Post], error)
	}
	Followers interface {
		Create(ctx context.Context, follower *Follower) error
		Delete(ctx context.Context, userId, followerId int64) error
		GetByUserId(ctx context.Context, userId int64) ([]*Follower, error)
		GetByFollowerId(ctx context.Context, followerId int64) ([]*Follower, error)
		GetAll(ctx context.Context) ([]*Follower, error)
	}
	Sessions interface {
		Create(ctx context.Context, session *Session) error
		GetById(ctx context.Context, sessionId int64) (*Session, error)
		GetByUserEmail(ctx context.Context, userEmail string) (*Session, error)
		RevokeSession(ctx context.Context, sessionId int64) error
		DeleteSession(ctx context.Context, sessionId int64) error
		UpdateSession(ctx context.Context, session *Session) error
	}
	Comments interface {
		Create(ctx context.Context, comment *Comment) error
		GetById(ctx context.Context, commentId int64) (*Comment, error)
		Update(ctx context.Context, comment *Comment) error
		Delete(ctx context.Context, commentId int64) error
		GetByPostId(ctx context.Context, postId int64, limit, offset int) (*PaginatedList[*Comment], error)
	}
	Tours interface {
		Create(ctx context.Context, tour *Tour) error
		GetByID(ctx context.Context, id int64) (*Tour, error)
		Update(ctx context.Context, tour *Tour) error
		Delete(ctx context.Context, id int64) error
		GetAll(ctx context.Context, limit, offset int) (*PaginatedList[*Tour], error)
		AddTourImagesUrl(ctx context.Context, tourId int64, imageUrl string) error
		GetTourImagesUrl(ctx context.Context, tourId int64) ([]string, error)
	}
	Events interface {
		GetByID(ctx context.Context, id int64) (*Event, error)
		Create(ctx context.Context, event *Event) error
		GetAll(ctx context.Context, limit, offset int) (*PaginatedList[*Event], error)
		Delete(ctx context.Context, id int64) error
	}
	Location interface {
		GetByID(ctx context.Context, id int64) (*Location, error)
	}
	Carts interface {
		GetCartItemByCartId(ctx context.Context, id int64) ([]*CartItem, error)
	}
	Equipments interface {
		GetByID(ctx context.Context, id int64) (*Equipment, error)
	}
	Subscriptions interface {
		GetUserSubscriptionByEmail(ctx context.Context, email string) (*Subscription, error)
		GetAll(ctx context.Context) ([]*Subscription, error)
		Create(ctx context.Context, subscription *Subscription) error
	}
	Bookmarks interface {
		Create(ctx context.Context, bookmark *Bookmark) error
		Delete(ctx context.Context, userID int64, hotspotLocationID int64) error
		GetByUserID(ctx context.Context, userID int64) ([]*Bookmark, error)
		GetTrendingBookmarks(ctx context.Context, limit, offset int) (*PaginatedList[*Bookmark], error)
		Exists(ctx context.Context, userID int64, hotspotLocationID string) (bool, error)
	}
	Species interface {
		GetRangeByScientificName(ctx context.Context, scientificName string) ([]RangeData, error)
	}
	Roles interface {
		GetByID(ctx context.Context, id int64) (*Role, error)
		AddUserToRole(ctx context.Context, userID int64, roleName string) error
	}
	Orders interface {
		Create(ctx context.Context, order *Order) error
		GetByGatewayOrderID(ctx context.Context, gatewayOrderID string) (*Order, error)
		UpdateStatus(ctx context.Context, id int64, status string) error
	}
}

func NewStore(db *sqlx.DB) *Storage {
	return &Storage{
		Users:         &UserStore{db},
		Posts:         &PostStore{db},
		Followers:     &FollowerStore{db},
		Sessions:      &SessionStore{db},
		Comments:      &CommentStore{db},
		Tours:         &TourStore{db},
		Events:        &EventStore{db},
		Location:      &LocationStore{db},
		Carts:         &CartStore{db},
		Equipments:    &EquipmentStore{db},
		Subscriptions: &SubscriptionStore{db},
		Bookmarks:     &BookmarksStore{db},
		Species:       &SpeciesStore{db},
		Roles:         &RoleStore{db},
		Orders:        &OrderStore{db},
	}
}

type PaginatedList[T any] struct {
	Items      []T   `json:"items"`
	TotalCount int64 `json:"total_count"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

func NewPaginatedList[T any](items []T, totalCount, limit, offset int) (*PaginatedList[T], error) {
	if limit <= 0 || limit > 100 {
		return nil, errors.New("limit must be between 1 and 100")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	if totalCount < 0 {
		return nil, errors.New("total count cannot be negative")
	}

	page := (offset / limit) + 1
	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))

	return &PaginatedList[T]{
		Items:      items,
		TotalCount: int64(totalCount),
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	}, nil
}