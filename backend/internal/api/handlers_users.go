package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"atrium-calls/backend/internal/auth"
	"atrium-calls/backend/internal/store"
)

// Only tenant_admin and platform_admin can manage team members.
const (
	roleTenantAdmin = "tenant_admin"
	roleTenantAgent = "tenant_agent"
)

func (s *Server) requireTenantAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.FromContext(r.Context())
		if claims.Role != roleTenantAdmin && claims.Role != "platform_admin" {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	})
}

func (s *Server) handleListTenantUsers(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	users, err := s.store.ListUsersByTenant(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type inviteUserInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type inviteUserResponse struct {
	User        store.User `json:"user"`
	TempPassword string    `json:"tempPassword"`
}

func (s *Server) handleInviteTenantUser(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input inviteUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	if input.Email == "" || input.Name == "" {
		writeError(w, http.StatusBadRequest, "email_and_name_required")
		return
	}
	if input.Role != roleTenantAdmin && input.Role != roleTenantAgent {
		input.Role = roleTenantAgent
	}
	taken, err := s.store.EmailTaken(r.Context(), input.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	if taken {
		writeError(w, http.StatusConflict, "email_already_exists")
		return
	}
	tempPwd := generateTempPassword()
	hash, err := auth.HashPassword(tempPwd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash_failed")
		return
	}
	user, err := s.store.InsertTenantUser(r.Context(), store.CreateUserInput{
		TenantID: tenantID, Email: input.Email, Name: input.Name, Role: input.Role, PasswordHash: hash,
	})
	if err != nil {
		s.logger.Error("invite user", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "user.invite", "user", user.ID, map[string]any{"email": user.Email, "role": user.Role})
	writeJSON(w, http.StatusCreated, inviteUserResponse{User: user, TempPassword: tempPwd})
}

type updateUserRoleInput struct {
	Role string `json:"role"`
}

func (s *Server) handleUpdateTenantUser(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	claims, _ := auth.FromContext(r.Context())
	if claims.Sub == id {
		writeError(w, http.StatusBadRequest, "cannot_modify_self")
		return
	}
	var input updateUserRoleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if input.Role != roleTenantAdmin && input.Role != roleTenantAgent {
		writeError(w, http.StatusBadRequest, "invalid_role")
		return
	}
	if err := s.store.UpdateTenantUserRole(r.Context(), tenantID, id, input.Role); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "user.role_change", "user", id, map[string]any{"role": input.Role})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteTenantUser(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	claims, _ := auth.FromContext(r.Context())
	if claims.Sub == id {
		writeError(w, http.StatusBadRequest, "cannot_delete_self")
		return
	}
	if err := s.store.DeleteTenantUser(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "user.delete", "user", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// generateTempPassword returns a short URL-safe random string used as a one-time password.
// The inviter shares it out-of-band; the user changes it on first login.
func generateTempPassword() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}
