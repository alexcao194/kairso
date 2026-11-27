package projectfeedback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/httpx"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrQuestionNotFound = errors.New("Không tìm thấy câu hỏi")
	ErrAlreadyAnswered  = errors.New("Câu hỏi này đã được trả lời")
)

const (
	maxQuestionLen    = 2000
	maxAnswerLen      = 4000
	maxAnswererFields = 150
)

type ProjectQuestionResponse struct {
	ID                string     `json:"id"`
	Question          string     `json:"question"`
	Answer            string     `json:"answer,omitempty"`
	AnsweredByName    string     `json:"answeredByName,omitempty"`
	AnsweredByCompany string     `json:"answeredByCompany,omitempty"`
	AnsweredAt        *time.Time `json:"answeredAt,omitempty" swaggertype:"string" format:"date-time"`
	CreatedAt         time.Time  `json:"createdAt" swaggertype:"string" format:"date-time"`
}

type CreateProjectQuestionRequest struct {
	Question string `json:"question"`
}

type AnswerProjectQuestionRequest struct {
	Answer          string `json:"answer"`
	AnswererName    string `json:"answererName"`
	AnswererCompany string `json:"answererCompany,omitempty"`
}

func textValue(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

func questionResponse(row database.ProjectQuestion) ProjectQuestionResponse {
	resp := ProjectQuestionResponse{
		ID:                pgutil.UUIDToString(row.ID),
		Question:          row.Question,
		Answer:            row.Answer.String,
		AnsweredByName:    row.AnsweredByName.String,
		AnsweredByCompany: row.AnsweredByCompany.String,
		CreatedAt:         row.CreatedAt.Time,
	}
	if row.AnsweredAt.Valid {
		t := row.AnsweredAt.Time
		resp.AnsweredAt = &t
	}
	return resp
}

// ---- repository ----

func (r *repository) CreateQuestion(ctx context.Context, projectID, userID, question string) (*ProjectQuestionResponse, error) {
	pid, err := pgutil.ParseUUID(projectID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateProjectQuestion(ctx, database.CreateProjectQuestionParams{ProjectID: pid, Question: question, CreatedByID: uid})
	if err != nil {
		return nil, err
	}
	resp := questionResponse(row)
	return &resp, nil
}

func (r *repository) ListQuestions(ctx context.Context, projectID string) ([]ProjectQuestionResponse, error) {
	pid, err := pgutil.ParseUUID(projectID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListProjectQuestions(ctx, pid)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectQuestionResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, questionResponse(row))
	}
	return out, nil
}

func (r *repository) GetQuestionProjectID(ctx context.Context, questionID string) (string, error) {
	qid, err := pgutil.ParseUUID(questionID)
	if err != nil {
		return "", ErrQuestionNotFound
	}
	row, err := r.queries.GetProjectQuestionByID(ctx, qid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrQuestionNotFound
	}
	if err != nil {
		return "", err
	}
	return pgutil.UUIDToString(row.ProjectID), nil
}

func (r *repository) UpdateQuestionText(ctx context.Context, questionID, question string) (*ProjectQuestionResponse, error) {
	qid, err := pgutil.ParseUUID(questionID)
	if err != nil {
		return nil, ErrQuestionNotFound
	}
	row, err := r.queries.UpdateProjectQuestionText(ctx, database.UpdateProjectQuestionTextParams{ID: qid, Question: question})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQuestionNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := questionResponse(row)
	return &resp, nil
}

func (r *repository) ClearQuestionAnswer(ctx context.Context, questionID string) (*ProjectQuestionResponse, error) {
	qid, err := pgutil.ParseUUID(questionID)
	if err != nil {
		return nil, ErrQuestionNotFound
	}
	row, err := r.queries.ClearProjectQuestionAnswer(ctx, qid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQuestionNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := questionResponse(row)
	return &resp, nil
}

func (r *repository) DeleteQuestion(ctx context.Context, questionID string) error {
	qid, err := pgutil.ParseUUID(questionID)
	if err != nil {
		return ErrQuestionNotFound
	}
	return r.queries.DeleteProjectQuestion(ctx, qid)
}

func (r *repository) AnswerQuestion(ctx context.Context, projectID, questionID string, req AnswerProjectQuestionRequest) (*ProjectQuestionResponse, error) {
	pid, err := pgutil.ParseUUID(projectID)
	if err != nil {
		return nil, err
	}
	qid, err := pgutil.ParseUUID(questionID)
	if err != nil {
		return nil, ErrQuestionNotFound
	}
	row, err := r.queries.AnswerProjectQuestion(ctx, database.AnswerProjectQuestionParams{
		ID: qid, ProjectID: pid,
		Answer:            textValue(req.Answer),
		AnsweredByName:    textValue(req.AnswererName),
		AnsweredByCompany: textValue(req.AnswererCompany),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the question is not in this project, or it was already answered.
		existing, getErr := r.queries.GetProjectQuestionByID(ctx, qid)
		if getErr == nil && existing.ProjectID == pid {
			return nil, ErrAlreadyAnswered
		}
		return nil, ErrQuestionNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := questionResponse(row)
	return &resp, nil
}

// ---- service ----

func (s *service) requireMember(ctx context.Context, projectID, callerID string) error {
	allowed, err := s.repo.CheckProjectMembership(ctx, projectID, callerID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func normalizeQuestion(q string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", fmt.Errorf("Câu hỏi là bắt buộc")
	}
	if len([]rune(q)) > maxQuestionLen {
		return "", fmt.Errorf("Câu hỏi tối đa %d ký tự", maxQuestionLen)
	}
	return q, nil
}

func (s *service) ListQuestions(ctx context.Context, projectID, callerID string) ([]ProjectQuestionResponse, error) {
	if err := s.requireMember(ctx, projectID, callerID); err != nil {
		return nil, err
	}
	return s.repo.ListQuestions(ctx, projectID)
}

func (s *service) CreateQuestion(ctx context.Context, projectID, callerID, question string) (*ProjectQuestionResponse, error) {
	if err := s.requireMember(ctx, projectID, callerID); err != nil {
		return nil, err
	}
	q, err := normalizeQuestion(question)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateQuestion(ctx, projectID, callerID, q)
}

func (s *service) memberOfQuestion(ctx context.Context, questionID, callerID string) error {
	projectID, err := s.repo.GetQuestionProjectID(ctx, questionID)
	if err != nil {
		return err
	}
	return s.requireMember(ctx, projectID, callerID)
}

func (s *service) UpdateQuestion(ctx context.Context, questionID, callerID, question string) (*ProjectQuestionResponse, error) {
	if err := s.memberOfQuestion(ctx, questionID, callerID); err != nil {
		return nil, err
	}
	q, err := normalizeQuestion(question)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateQuestionText(ctx, questionID, q)
}

func (s *service) ClearQuestionAnswer(ctx context.Context, questionID, callerID string) (*ProjectQuestionResponse, error) {
	if err := s.memberOfQuestion(ctx, questionID, callerID); err != nil {
		return nil, err
	}
	return s.repo.ClearQuestionAnswer(ctx, questionID)
}

func (s *service) DeleteQuestion(ctx context.Context, questionID, callerID string) error {
	if err := s.memberOfQuestion(ctx, questionID, callerID); err != nil {
		return err
	}
	return s.repo.DeleteQuestion(ctx, questionID)
}

func (s *service) ListPublicQuestions(ctx context.Context, token string) ([]ProjectQuestionResponse, error) {
	project, projectID, err := s.repo.GetPublicProjectByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !project.Enabled {
		return nil, ErrDisabled
	}
	return s.repo.ListQuestions(ctx, projectID)
}

func (s *service) AnswerPublicQuestion(ctx context.Context, token, questionID string, req AnswerProjectQuestionRequest) (*ProjectQuestionResponse, error) {
	project, projectID, err := s.repo.GetPublicProjectByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !project.Enabled {
		return nil, ErrDisabled
	}
	req.Answer = strings.TrimSpace(req.Answer)
	req.AnswererName = strings.TrimSpace(req.AnswererName)
	req.AnswererCompany = strings.TrimSpace(req.AnswererCompany)
	if req.Answer == "" {
		return nil, fmt.Errorf("Câu trả lời là bắt buộc")
	}
	if len([]rune(req.Answer)) > maxAnswerLen {
		return nil, fmt.Errorf("Câu trả lời tối đa %d ký tự", maxAnswerLen)
	}
	if req.AnswererName == "" {
		return nil, fmt.Errorf("answererName là bắt buộc")
	}
	if len([]rune(req.AnswererName)) > maxAnswererFields || len([]rune(req.AnswererCompany)) > maxAnswererFields {
		return nil, fmt.Errorf("Tên và công ty tối đa %d ký tự", maxAnswererFields)
	}
	return s.repo.AnswerQuestion(ctx, projectID, questionID, req)
}

// ---- member handlers ----

func (h *Handler) registerQuestionRoutes(mux *http.ServeMux, authMid *middleware.AuthMiddleware) {
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/projects/{projectId}/questions", authMid.Handle(h.ListQuestions))
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/projects/{projectId}/questions", authMid.Handle(h.CreateQuestion))
	mux.HandleFunc("PATCH /v1/project-questions/{questionId}", authMid.Handle(h.UpdateQuestion))
	mux.HandleFunc("DELETE /v1/project-questions/{questionId}/answer", authMid.Handle(h.ClearQuestionAnswer))
	mux.HandleFunc("DELETE /v1/project-questions/{questionId}", authMid.Handle(h.DeleteQuestion))
}

func questionError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, ErrQuestionNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrAlreadyAnswered):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, "Bạn không có quyền thực hiện thao tác này")
	default:
		httpx.Error(w, http.StatusInternalServerError, fmt.Sprintf("Không thể %s: %s", action, err.Error()))
	}
}

// validation errors from the service are plain errors; surface them as 400.
func isValidationError(err error) bool {
	if err == nil || errors.Is(err, ErrForbidden) || errors.Is(err, ErrQuestionNotFound) || errors.Is(err, ErrAlreadyAnswered) || errors.Is(err, ErrDisabled) {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "bắt buộc") || strings.Contains(msg, "tối đa")
}

func writeQuestionError(w http.ResponseWriter, err error, action string) {
	if isValidationError(err) {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	questionError(w, err, action)
}

// @Summary List Project Questions
// @Tags ProjectQuestion
// @Produce json
// @Security BearerAuth
// @Param workspaceId path string true "Workspace ID"
// @Param projectId path string true "Project ID"
// @Success 200 {array} ProjectQuestionResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/projects/{projectId}/questions [get]
func (h *Handler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.RequireUser(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.ListQuestions(r.Context(), r.PathValue("projectId"), callerID)
	if err != nil {
		writeQuestionError(w, err, "lấy danh sách câu hỏi")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Create Project Question
// @Description Members write a question that people with the public link can answer.
// @Tags ProjectQuestion
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param workspaceId path string true "Workspace ID"
// @Param projectId path string true "Project ID"
// @Param request body CreateProjectQuestionRequest true "Question"
// @Success 201 {object} ProjectQuestionResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/projects/{projectId}/questions [post]
func (h *Handler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.RequireUser(w, r)
	if !ok {
		return
	}
	var req CreateProjectQuestionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("Nội dung yêu cầu không hợp lệ: %s", err.Error()))
		return
	}
	resp, err := h.svc.CreateQuestion(r.Context(), r.PathValue("projectId"), callerID, req.Question)
	if err != nil {
		writeQuestionError(w, err, "tạo câu hỏi")
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// @Summary Update Project Question
// @Tags ProjectQuestion
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param questionId path string true "Question ID"
// @Param request body CreateProjectQuestionRequest true "Question"
// @Success 200 {object} ProjectQuestionResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/project-questions/{questionId} [patch]
func (h *Handler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.RequireUser(w, r)
	if !ok {
		return
	}
	var req CreateProjectQuestionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("Nội dung yêu cầu không hợp lệ: %s", err.Error()))
		return
	}
	resp, err := h.svc.UpdateQuestion(r.Context(), r.PathValue("questionId"), callerID, req.Question)
	if err != nil {
		writeQuestionError(w, err, "cập nhật câu hỏi")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Clear Project Question Answer
// @Description Removes the answer so the question can be answered again from the public link.
// @Tags ProjectQuestion
// @Produce json
// @Security BearerAuth
// @Param questionId path string true "Question ID"
// @Success 200 {object} ProjectQuestionResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/project-questions/{questionId}/answer [delete]
func (h *Handler) ClearQuestionAnswer(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.RequireUser(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.ClearQuestionAnswer(r.Context(), r.PathValue("questionId"), callerID)
	if err != nil {
		writeQuestionError(w, err, "xoá câu trả lời")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Delete Project Question
// @Tags ProjectQuestion
// @Security BearerAuth
// @Param questionId path string true "Question ID"
// @Success 204
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/project-questions/{questionId} [delete]
func (h *Handler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.RequireUser(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteQuestion(r.Context(), r.PathValue("questionId"), callerID); err != nil {
		writeQuestionError(w, err, "xoá câu hỏi")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- public handlers ----

func (h *PublicHandler) registerQuestionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /public/projects/{token}/questions", h.ListQuestions)
	mux.HandleFunc("POST /public/projects/{token}/questions/{questionId}/answer", h.AnswerQuestion)
}

// @Summary List Public Questions
// @Description No authentication required — gated only by the unguessable token.
// @Tags PublicFeedback
// @Produce json
// @Param token path string true "Public feedback token"
// @Success 200 {array} ProjectQuestionResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /public/projects/{token}/questions [get]
func (h *PublicHandler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListPublicQuestions(r.Context(), r.PathValue("token"))
	if err != nil {
		h.handleError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Answer Public Question
// @Description No authentication required — gated only by the unguessable token. Only unanswered questions can be answered.
// @Tags PublicFeedback
// @Accept json
// @Produce json
// @Param token path string true "Public feedback token"
// @Param questionId path string true "Question ID"
// @Param request body AnswerProjectQuestionRequest true "Answer"
// @Success 200 {object} ProjectQuestionResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Router /public/projects/{token}/questions/{questionId}/answer [post]
func (h *PublicHandler) AnswerQuestion(w http.ResponseWriter, r *http.Request) {
	var req AnswerProjectQuestionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("Nội dung yêu cầu không hợp lệ: %s", err.Error()))
		return
	}
	resp, err := h.svc.AnswerPublicQuestion(r.Context(), r.PathValue("token"), r.PathValue("questionId"), req)
	if err != nil {
		switch {
		case isValidationError(err):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrAlreadyAnswered):
			httpx.Error(w, http.StatusConflict, err.Error())
		default:
			h.handleError(w, err)
		}
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
