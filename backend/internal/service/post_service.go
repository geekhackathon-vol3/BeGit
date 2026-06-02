package service

import (
	"context"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
	"github.com/irj0927/begit/pkg/r2"
)

// feedPhotoURLTTL はフィードで返す presigned GET URL の有効期限
const feedPhotoURLTTL = time.Hour

// CreatePostRequest は投稿作成リクエスト
type CreatePostRequest struct {
	Body           *string
	NotificationID *int64
	AccessToken    string
	GitHubLogin    string
	RepoFullName   string
}

// PostService は投稿・フィードサービスインターフェース
type PostService interface {
	CreatePost(ctx context.Context, req CreatePostRequest, groupID, userID int64) (*model.Post, error)
	ListPosts(ctx context.Context, groupID, userID int64) ([]model.PostFeed, error)
}

// postService は PostService インターフェースの実装
type postService struct {
	githubClient githubpkg.Client
	sprintRepo   repository.SprintRepository
	postRepo     repository.PostRepository
	groupRepo    repository.GroupRepository
	photoRepo    repository.PhotoRepository
	r2Client     r2.Client
}

// NewPostService は PostService を作成する。
// photoRepo / r2Client はフィードに写真の presigned URL を付与するために使う（nil 可）。
func NewPostService(
	githubClient githubpkg.Client,
	sprintRepo repository.SprintRepository,
	postRepo repository.PostRepository,
	groupRepo repository.GroupRepository,
	photoRepo repository.PhotoRepository,
	r2Client r2.Client,
) PostService {
	return &postService{
		githubClient: githubClient,
		sprintRepo:   sprintRepo,
		postRepo:     postRepo,
		groupRepo:    groupRepo,
		photoRepo:    photoRepo,
		r2Client:     r2Client,
	}
}

// CreatePost は GitHub コミット情報を取得して posts テーブルに INSERT する
func (s *postService) CreatePost(ctx context.Context, req CreatePostRequest, groupID, userID int64) (*model.Post, error) {
	// GitHub クライアントが未設定の場合は ErrExternalAPI を返す
	if s.githubClient == nil {
		return nil, fmt.Errorf("%w: github client not configured", ErrExternalAPI)
	}

	// Step 1: GitHub からコミット情報を取得
	commitSummary, err := s.githubClient.GetRecentCommits(ctx, req.RepoFullName, req.GitHubLogin, req.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get recent commits: %v", ErrExternalAPI, err)
	}

	// Step 2: posts テーブルに INSERT
	repoFullName := commitSummary.RepoFullName
	latestCommitMsg := commitSummary.LatestCommitMessage

	post := &model.Post{
		NotificationID:      req.NotificationID,
		UserID:              userID,
		GroupID:             groupID,
		PostType:            "commit",
		Body:                req.Body,
		RepoFullName:        &repoFullName,
		CommitCount:         commitSummary.CommitCount,
		Additions:           commitSummary.Additions,
		Deletions:           commitSummary.Deletions,
		LatestCommitMessage: &latestCommitMsg,
	}

	created, err := s.postRepo.Create(ctx, post)
	if err != nil {
		return nil, fmt.Errorf("post_service: CreatePost failed: %w", err)
	}

	return created, nil
}

// ListPosts はグループのフィードを取得し、リクエストユーザーの投稿状況によってぼかし制御を適用する
func (s *postService) ListPosts(ctx context.Context, groupID, userID int64) ([]model.PostFeed, error) {
	// Step 1: 現在のスプリントを取得
	var sprintID int64
	if s.sprintRepo != nil {
		sprint, err := s.sprintRepo.GetCurrentSprint(ctx, groupID)
		if err == nil {
			sprintID = sprint.ID
		}
	}

	// Step 2: リクエストユーザーが現スプリントで投稿済みかどうか確認
	hasPosted := false
	if sprintID > 0 && s.postRepo != nil {
		posted, err := s.postRepo.HasPostedInSprint(ctx, userID, sprintID)
		if err == nil {
			hasPosted = posted
		}
	}

	// Step 3: グループの投稿一覧を取得
	posts, err := s.postRepo.ListByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("post_service: ListPosts failed: %w", err)
	}

	// Step 4: グループメンバー情報を取得（Login/AvatarURL 付与のため）
	memberMap := make(map[int64]model.GroupMember)
	if s.groupRepo != nil {
		members, err := s.groupRepo.GetMembers(ctx, groupID)
		if err == nil {
			for _, m := range members {
				memberMap[m.UserID] = m
			}
		}
	}

	// Step 5: 投稿に紐づく写真をまとめて取得（N+1 回避）
	photoMap := make(map[int64][]model.Photo)
	if s.photoRepo != nil && len(posts) > 0 {
		postIDs := make([]int64, 0, len(posts))
		for _, p := range posts {
			postIDs = append(postIDs, p.ID)
		}
		if m, err := s.photoRepo.ListByPostIDs(ctx, postIDs); err == nil {
			photoMap = m
		}
	}

	// Step 6: PostFeed を構築し、ぼかし制御を適用
	feeds := make([]model.PostFeed, 0, len(posts))
	for _, post := range posts {
		feed := model.PostFeed{
			Post:    post,
			Blurred: false,
		}

		// メンバー情報を付与
		if member, ok := memberMap[post.UserID]; ok {
			feed.Login = member.Login
			feed.AvatarURL = member.AvatarURL
		}

		// ぼかし制御: リクエストユーザーが未投稿かつ他メンバーの投稿
		if !hasPosted && post.UserID != userID {
			feed.Blurred = true
			feed.Body = nil
			feed.RepoFullName = nil
			feed.LatestCommitMessage = nil
			// ぼかし対象は写真も返さない
		} else {
			feed.Photos = s.buildFeedPhotos(photoMap[post.ID])
		}

		feeds = append(feeds, feed)
	}

	return feeds, nil
}

// buildFeedPhotos は写真に presigned GET URL を付与してフィード用に変換する。
// r2Client が未設定、または URL 生成に失敗した写真はスキップする。
func (s *postService) buildFeedPhotos(photos []model.Photo) []model.FeedPhoto {
	if s.r2Client == nil || len(photos) == 0 {
		return nil
	}
	out := make([]model.FeedPhoto, 0, len(photos))
	for _, p := range photos {
		url, err := s.r2Client.PresignGetURL(p.R2Key, feedPhotoURLTTL)
		if err != nil {
			continue
		}
		out = append(out, model.FeedPhoto{
			ID:        p.ID,
			PhotoType: p.PhotoType,
			URL:       url,
		})
	}
	return out
}
