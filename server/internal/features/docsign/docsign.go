// Package docsign stores the name-and-date stamps shown on quotes and meeting minutes.
package docsign

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	TypeQuote   = "quote"
	TypeMeeting = "meeting"

	RolePreparer   = "preparer"
	RoleController = "controller"
)

var (
	ErrAlreadySigned = errors.New("Vai trò này đã được ký")
	ErrNoSignature   = errors.New("Chưa có chữ ký ở vai trò này")
)

// Signature is one stamp: who signed, under which role, and when.
type Signature struct {
	Role     string    `json:"role" enums:"preparer,controller"`
	UserID   string    `json:"userId"`
	Name     string    `json:"name"`
	SignedAt time.Time `json:"signedAt" swaggertype:"string" format:"date-time"`
}

type Store struct {
	q *database.Queries
}

func NewStore(q *database.Queries) *Store { return &Store{q: q} }

// ValidRole reports whether role is one of the two stamp roles.
func ValidRole(role string) bool { return role == RolePreparer || role == RoleController }

// List returns the stamps on a document, oldest first. Never nil.
func (s *Store) List(ctx context.Context, docType string, docID pgtype.UUID) ([]Signature, error) {
	rows, err := s.q.ListDocumentSignatures(ctx, database.ListDocumentSignaturesParams{DocType: docType, DocID: docID})
	if err != nil {
		return nil, err
	}
	out := make([]Signature, 0, len(rows))
	for _, r := range rows {
		out = append(out, Signature{Role: r.Role, UserID: pgutil.UUIDToString(r.UserID), Name: r.Name, SignedAt: r.SignedAt.Time})
	}
	return out, nil
}

// Find returns the stamp for one role, if any.
func (s *Store) Find(ctx context.Context, docType string, docID pgtype.UUID, role string) (Signature, bool, error) {
	all, err := s.List(ctx, docType, docID)
	if err != nil {
		return Signature{}, false, err
	}
	for _, sig := range all {
		if sig.Role == role {
			return sig, true, nil
		}
	}
	return Signature{}, false, nil
}

// Sign stamps the document with the caller's current name. A role can be stamped once; remove it first to restamp.
func (s *Store) Sign(ctx context.Context, docType string, docID pgtype.UUID, role string, userID pgtype.UUID) error {
	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(u.Name)
	if name == "" {
		name = u.Alias
	}
	n, err := s.q.InsertDocumentSignature(ctx, database.InsertDocumentSignatureParams{
		DocType: docType, DocID: docID, Role: role, UserID: userID, Name: name,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAlreadySigned
	}
	return nil
}

func (s *Store) Unsign(ctx context.Context, docType string, docID pgtype.UUID, role string) error {
	return s.q.DeleteDocumentSignature(ctx, database.DeleteDocumentSignatureParams{DocType: docType, DocID: docID, Role: role})
}

// Clear drops every stamp on a document (used when the document is deleted).
func (s *Store) Clear(ctx context.Context, docType string, docID pgtype.UUID) error {
	return s.q.DeleteDocumentSignatures(ctx, database.DeleteDocumentSignaturesParams{DocType: docType, DocID: docID})
}
