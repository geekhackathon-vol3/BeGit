package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	Body              *string
	NotificationID    *int64
	PostType          string
	ContentSource     string
	CommitSHA         *string
	PullRequestNumber *int
	AccessToken       string
	GitHubLogin       string
	RepoFullName      string
}

// ConfirmPostRequest は下書き確定リクエスト（確定時に本文を上書きできる）
type ConfirmPostRequest struct {
	Body *string
}

// PostService は投稿・フィードサービスインターフェース
type PostService interface {
	CreatePost(ctx context.Context, req CreatePostRequest, groupID, userID int64) (*model.Post, error)
	ListPosts(ctx context.Context, groupID, userID int64) ([]model.PostFeed, error)
	// GetDraft は下書き投稿を取得する（② プレフィル元）。本人以外/別グループ/draft でない場合はエラー。
	GetDraft(ctx context.Context, groupID, postID, userID int64) (*model.Post, error)
	// ConfirmPost は下書きを確定（is_draft=0）してフィード表示可能にする。べき等。
	ConfirmPost(ctx context.Context, req ConfirmPostRequest, groupID, postID, userID int64) (*model.Post, error)
	// DeletePost は本人の投稿を削除する。GitHub上のデータは変更しない。
	DeletePost(ctx context.Context, groupID, postID, userID int64) error
}

// postService は PostService インターフェースの実装
type postService struct {
	githubClient githubpkg.Client
	sprintRepo   repository.SprintRepository
	postRepo     repository.PostRepository
	groupRepo    repository.GroupRepository
	photoRepo    repository.PhotoRepository
	r2Client     r2.Client
	notifRepo    repository.NotificationRepository
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
	notifRepos ...repository.NotificationRepository,
) PostService {
	var notifRepo repository.NotificationRepository
	if len(notifRepos) > 0 {
		notifRepo = notifRepos[0]
	}
	return &postService{
		githubClient: githubClient,
		sprintRepo:   sprintRepo,
		postRepo:     postRepo,
		groupRepo:    groupRepo,
		photoRepo:    photoRepo,
		r2Client:     r2Client,
		notifRepo:    notifRepo,
	}
}

// CreatePost は投稿種別に対応する GitHub 情報を取得して posts テーブルに INSERT する。
func (s *postService) CreatePost(ctx context.Context, req CreatePostRequest, groupID, userID int64) (*model.Post, error) {
	postType := req.PostType
	if postType == "" {
		postType = "commit"
	}
	if postType != "commit" && postType != "pull_request" && postType != "memo" {
		return nil, fmt.Errorf("%w: unsupported post type %q", ErrValidation, postType)
	}
	contentSource := req.ContentSource
	if contentSource == "" {
		contentSource = "github"
	}
	if postType == "memo" {
		contentSource = "manual"
	}
	if contentSource != "github" && contentSource != "manual" {
		return nil, fmt.Errorf("%w: unsupported content source %q", ErrValidation, contentSource)
	}
	if contentSource == "manual" && (req.Body == nil || strings.TrimSpace(*req.Body) == "") {
		return nil, fmt.Errorf("%w: comment body is required for manual posts", ErrValidation)
	}
	if req.NotificationID != nil && s.notifRepo != nil {
		if err := s.validateNotificationPost(ctx, groupID, *req.NotificationID); err != nil {
			return nil, err
		}
	}

	repoFullName := req.RepoFullName
	post := &model.Post{
		NotificationID: req.NotificationID,
		UserID:         userID,
		GroupID:        groupID,
		PostType:       postType,
		Body:           req.Body,
		RepoFullName:   &repoFullName,
	}

	if contentSource == "github" && s.githubClient == nil {
		return nil, fmt.Errorf("%w: github client not configured", ErrExternalAPI)
	}

	switch postType {
	case "commit":
		if contentSource == "manual" {
			break
		}
		if req.CommitSHA != nil && strings.TrimSpace(*req.CommitSHA) != "" {
			commit, err := s.githubClient.GetCommit(ctx, req.RepoFullName, strings.TrimSpace(*req.CommitSHA), req.AccessToken)
			if err != nil {
				return nil, fmt.Errorf("%w: failed to get commit: %v", ErrExternalAPI, err)
			}
			if commit.AuthorLogin == "" || !strings.EqualFold(commit.AuthorLogin, req.GitHubLogin) {
				return nil, fmt.Errorf("%w: selected commit is not authored by the current user", ErrValidation)
			}
			post.CommitCount = 1
			post.Additions = commit.Additions
			post.Deletions = commit.Deletions
			post.LatestCommitMessage = strOrNil(commit.Message)
			break
		}
		commitSummary, err := s.githubClient.GetRecentCommits(ctx, req.RepoFullName, req.GitHubLogin, req.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to get recent commits: %v", ErrExternalAPI, err)
		}
		post.RepoFullName = strOrNil(commitSummary.RepoFullName)
		post.CommitCount = commitSummary.CommitCount
		post.Additions = commitSummary.Additions
		post.Deletions = commitSummary.Deletions
		post.LatestCommitMessage = strOrNil(commitSummary.LatestCommitMessage)
	case "pull_request":
		if contentSource == "manual" {
			break
		}
		if req.PullRequestNumber != nil && *req.PullRequestNumber > 0 {
			pullRequest, err := s.githubClient.GetPullRequest(ctx, req.RepoFullName, *req.PullRequestNumber, req.AccessToken)
			if err != nil {
				return nil, fmt.Errorf("%w: failed to get pull request: %v", ErrExternalAPI, err)
			}
			if !strings.EqualFold(pullRequest.AuthorLogin, req.GitHubLogin) {
				return nil, fmt.Errorf("%w: selected pull request is not authored by the current user", ErrValidation)
			}
			post.LatestCommitMessage = strOrNil(fmt.Sprintf("PR #%d: %s", pullRequest.Number, pullRequest.Title))
			break
		}
		pullRequest, err := s.githubClient.GetLatestPullRequest(ctx, req.RepoFullName, req.GitHubLogin, req.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to get latest pull request: %v", ErrExternalAPI, err)
		}
		post.RepoFullName = strOrNil(pullRequest.RepoFullName)
		if pullRequest.Number > 0 && pullRequest.Title != "" {
			post.LatestCommitMessage = strOrNil(fmt.Sprintf("PR #%d: %s", pullRequest.Number, pullRequest.Title))
		}
	}

	created, err := s.postRepo.Create(ctx, post)
	if err != nil {
		return nil, fmt.Errorf("post_service: CreatePost failed: %w", err)
	}

	return created, nil
}

// validateNotificationPost は停止・期限切れのBeGit Timeへの新規投稿を拒否する。
func (s *postService) validateNotificationPost(ctx context.Context, groupID, notificationID int64) error {
	notif, err := s.notifRepo.GetByID(ctx, notificationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: notification not found", ErrValidation)
		}
		return fmt.Errorf("post_service: validate notification failed: %w", err)
	}
	sprint, err := s.sprintRepo.GetByID(ctx, notif.SprintID)
	if err != nil {
		return fmt.Errorf("post_service: validate notification sprint failed: %w", err)
	}
	if sprint.GroupID != groupID {
		return fmt.Errorf("%w: notification does not belong to group", ErrValidation)
	}
	if notif.StoppedAt != nil || !time.Now().UTC().Before(notif.SentAt.Add(challengeWindow)) {
		return fmt.Errorf("%w: notification is no longer accepting posts", ErrConflict)
	}
	return nil
}

// GetDraft は下書き投稿を取得する（② プレフィル元）。
// 別グループ → ErrNotFound、本人以外 → ErrForbidden、draft でない → ErrNotFound。
func (s *postService) GetDraft(ctx context.Context, groupID, postID, userID int64) (*model.Post, error) {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("post_service: GetDraft failed: %w", err)
	}
	if post.GroupID != groupID {
		return nil, ErrNotFound
	}
	if post.UserID != userID {
		return nil, ErrForbidden
	}
	if !post.IsDraft {
		// 既に確定済み（draft ではない）
		return nil, ErrNotFound
	}
	return post, nil
}

// ConfirmPost は下書きを確定（is_draft=0）してフィード表示可能にする。べき等（既確定でもエラーにしない）。
func (s *postService) ConfirmPost(ctx context.Context, req ConfirmPostRequest, groupID, postID, userID int64) (*model.Post, error) {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("post_service: ConfirmPost failed: %w", err)
	}
	if post.GroupID != groupID {
		return nil, ErrNotFound
	}
	if post.UserID != userID {
		return nil, ErrForbidden
	}

	// 既に確定済み（is_draft=0）の場合は本文更新をスキップし、現在の状態を返す（べき等）。
	if !post.IsDraft {
		return post, nil
	}

	// 下書き状態の場合のみ本文上書きと確定処理を実行
	if req.Body != nil {
		if err := s.postRepo.UpdateBody(ctx, postID, *req.Body); err != nil {
			return nil, fmt.Errorf("post_service: ConfirmPost UpdateBody failed: %w", err)
		}
	}

	// draft 解除
	if err := s.postRepo.ConfirmDraft(ctx, postID); err != nil {
		return nil, fmt.Errorf("post_service: ConfirmPost ConfirmDraft failed: %w", err)
	}

	// 確定後の状態を取得して返す
	confirmed, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("post_service: ConfirmPost re-fetch failed: %w", err)
	}
	return confirmed, nil
}

// DeletePost は本人の投稿を削除する。GitHub上のcommit/PRは削除しない。
func (s *postService) DeletePost(ctx context.Context, groupID, postID, userID int64) error {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("post_service: DeletePost lookup failed: %w", err)
	}
	if post.GroupID != groupID {
		return ErrNotFound
	}
	if post.UserID != userID {
		return ErrForbidden
	}

	// R2上の写真をDBレコードより先に削除する。失敗時は投稿を残して再試行可能にする。
	if s.photoRepo != nil {
		photos, err := s.photoRepo.ListByPostID(ctx, postID)
		if err != nil {
			return fmt.Errorf("post_service: DeletePost list photos failed: %w", err)
		}
		if s.r2Client != nil {
			for _, photo := range photos {
				if err := s.r2Client.DeleteObject(ctx, photo.R2Key); err != nil {
					return fmt.Errorf("%w: failed to delete photo", ErrExternalAPI)
				}
			}
		}
	}

	deleter, ok := s.postRepo.(interface {
		Delete(context.Context, int64) error
	})
	if !ok {
		return fmt.Errorf("post_service: DeletePost is not supported by repository")
	}
	if err := deleter.Delete(ctx, postID); err != nil {
		return fmt.Errorf("post_service: DeletePost failed: %w", err)
	}
	return nil
}

// ListPosts はグループのフィードを取得し、最新の未回答 BeGit Time に対してロック制御を適用する。
// 自分が最後に投稿した通知より前の投稿は過去分として常に表示する。より新しい通知に紐づく
// 他メンバーの投稿だけを隠すため、次の通知で自分が投稿すれば、それ以前に隠れていた投稿も表示される。
func (s *postService) ListPosts(ctx context.Context, groupID, userID int64) ([]model.PostFeed, error) {
	// Step 1: グループの投稿一覧を取得
	posts, err := s.postRepo.ListByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("post_service: ListPosts failed: %w", err)
	}

	// Step 2: 閲覧者が最後に投稿した通知IDを求める。
	// notification ID は発行順に増えるため、これより前の通知に紐づく投稿は過去分として公開する。
	// 取得済み投稿だけで判定するので追加DB問い合わせは不要。
	latestPostedNotificationID := int64(0)
	for _, post := range posts {
		if post.UserID != userID || post.NotificationID == nil || isMissedPost(&post) {
			continue
		}
		if *post.NotificationID > latestPostedNotificationID {
			latestPostedNotificationID = *post.NotificationID
		}
	}

	// Step 3: グループメンバー情報を取得（Login/AvatarURL 付与のため）
	memberMap := make(map[int64]model.GroupMember)
	if s.groupRepo != nil {
		members, err := s.groupRepo.GetMembers(ctx, groupID)
		if err == nil {
			for _, m := range members {
				memberMap[m.UserID] = m
			}
		}
	}

	// Step 4: 投稿に紐づく写真をまとめて取得（N+1 回避）
	photoMap := make(map[int64][]model.Photo)
	if s.photoRepo != nil && len(posts) > 0 {
		postIDs := make([]int64, 0, len(posts))
		for _, p := range posts {
			postIDs = append(postIDs, p.ID)
		}
		m, err := s.photoRepo.ListByPostIDs(ctx, postIDs)
		if err != nil {
			return nil, fmt.Errorf("post_service: ListByPostIDs failed: %w", err)
		}
		photoMap = m
	}

	// Step 5: PostFeed を構築し、ロック制御を適用
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

		// 最後に自分が投稿した通知より新しい投稿のみロックする。
		// 通知以前の投稿（notification_idなし）と自分の投稿は常に表示する。
		isLocked := post.UserID != userID && post.NotificationID != nil &&
			*post.NotificationID > latestPostedNotificationID
		if isLocked {
			feed.Blurred = true
			feed.Body = nil
			feed.RepoFullName = nil
			feed.LatestCommitMessage = nil
			feed.BranchName = nil
			feed.CommitCount = 0
			feed.Additions = 0
			feed.Deletions = 0
			feed.Status = nil
			// ロック対象は写真URLも返さない
		} else {
			feed.Photos = s.buildFeedPhotos(photoMap[post.ID])
		}

		feeds = append(feeds, feed)
	}

	return feeds, nil
}

func isMissedPost(post *model.Post) bool {
	return post.Status != nil && *post.Status == "missed"
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
