package user

import (
	"context"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/pkg/realtime"
)

type Service interface {
	GetMe(ctx context.Context, userID string) (*UserResponse, error)
	GetUserByAlias(ctx context.Context, alias string) (*UserResponse, error)
	UpdateMe(ctx context.Context, userID string, req UpdateMeRequest) (*UserResponse, error)
}

type service struct {
	repo     Repository
	realtime realtime.Broadcaster
}

func NewService(repo Repository) *service {
	return &service{repo: repo}
}

// SetRealtime wires the WebSocket broadcaster in; it's separate from
// NewService so existing test call sites don't also need a broadcaster.
func (s *service) SetRealtime(rt realtime.Broadcaster) {
	s.realtime = rt
}

// broadcastToUserWorkspaces fans the event out to every workspace the user
// belongs to, since a profile change (name/avatar) is visible everywhere
// the user appears as a member.
func (s *service) broadcastToUserWorkspaces(ctx context.Context, userID, eventType string, entity any) {
	if s.realtime == nil {
		return
	}
	workspaceIDs, err := s.repo.ListWorkspaceIDsByUser(ctx, userID)
	if err != nil {
		return
	}
	for _, workspaceID := range workspaceIDs {
		s.realtime.Broadcast(realtime.Event{
			Type:        eventType,
			WorkspaceID: workspaceID,
			EntityID:    userID,
			ActorID:     userID,
			ActorConnID: middleware.GetConnIDFromContext(ctx),
			Entity:      entity,
		})
	}
}

func (s *service) GetMe(ctx context.Context, userID string) (*UserResponse, error) {
	return s.repo.GetUser(ctx, userID)
}

func (s *service) GetUserByAlias(ctx context.Context, alias string) (*UserResponse, error) {
	return s.repo.GetUserByAlias(ctx, alias)
}

func (s *service) UpdateMe(ctx context.Context, userID string, req UpdateMeRequest) (*UserResponse, error) {
	user, err := s.repo.UpdateUser(ctx, userID, req)
	if err != nil {
		return nil, err
	}
	s.broadcastToUserWorkspaces(ctx, userID, "user.updated", user)
	return user, nil
}
