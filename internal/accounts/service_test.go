package accounts

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time          { return c.t }
func (c *testClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestService(t *testing.T) (*Service, *testClock) {
	t.Helper()
	clock := &testClock{t: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	s, err := Open(":memory:", Options{
		SigningKey:       []byte("0123456789abcdef0123456789abcdef"),
		PublicURL:        "https://clipmind.test",
		PBKDF2Iterations: 1000,
		Now:              clock.now,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, clock
}

var linkToken = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// lastToken reads the token from the newest email sent to the address.
func lastToken(t *testing.T, s *Service, to string) string {
	t.Helper()
	emails, err := s.PendingEmails(context.Background(), 100)
	if err != nil {
		t.Fatalf("PendingEmails: %v", err)
	}
	for i := len(emails) - 1; i >= 0; i-- {
		if emails[i].To == to {
			if m := linkToken.FindStringSubmatch(emails[i].Body); m != nil {
				return m[1]
			}
		}
	}
	t.Fatalf("no email with a token for %s", to)
	return ""
}

func confirmedUser(t *testing.T, s *Service, email string) User {
	t.Helper()
	ctx := context.Background()
	if err := s.Signup(ctx, SignupInput{Email: email, Password: "correct horse battery", DisplayName: "Test"}); err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if err := s.ConfirmEmail(ctx, lastToken(t, s, email)); err != nil {
		t.Fatalf("ConfirmEmail: %v", err)
	}
	var u User
	if err := s.db.QueryRow(`SELECT id, email, display_name, created_at FROM users WHERE email = ?`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt); err != nil {
		t.Fatalf("lookup: %v", err)
	}
	return u
}

func TestSignupConfirmLogin(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	if err := s.Signup(ctx, SignupInput{Email: " Ana@Example.com ", Password: "correct horse battery"}); err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if _, err := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "correct horse battery"}); !errors.Is(err, ErrEmailNotConfirmed) {
		t.Fatalf("login before confirming = %v, want ErrEmailNotConfirmed", err)
	}
	token := lastToken(t, s, "ana@example.com")
	if err := s.ConfirmEmail(ctx, token); err != nil {
		t.Fatalf("ConfirmEmail: %v", err)
	}
	if err := s.ConfirmEmail(ctx, token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second ConfirmEmail = %v, want ErrTokenInvalid", err)
	}
	tokens, err := s.Login(ctx, LoginInput{Email: "ANA@example.com", Password: "correct horse battery", Device: "PC"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.WorkspaceID == "" {
		t.Fatal("login did not pick the personal workspace")
	}
	p, err := s.Authenticate(ctx, tokens.Access)
	if err != nil || p.Role != RoleOwner || p.WorkspaceID != tokens.WorkspaceID {
		t.Fatalf("Authenticate = %+v, %v", p, err)
	}
}

func TestSignupDoesNotRevealExistingAccounts(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	confirmedUser(t, s, "ana@example.com")
	if err := s.Signup(ctx, SignupInput{Email: "ana@example.com", Password: "another long password"}); err != nil {
		t.Fatalf("Signup of an existing address = %v, want nil", err)
	}
	emails, _ := s.PendingEmails(ctx, 100)
	last := emails[len(emails)-1]
	if !strings.Contains(last.Subject, "Ya tienes una cuenta") || strings.Contains(last.Body, "token=") {
		t.Fatalf("email for an existing address = %+v", last)
	}
}

func TestSignupValidation(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	for _, in := range []SignupInput{
		{Email: "not-an-email", Password: "correct horse battery"},
		{Email: "Ana <ana@example.com>", Password: "correct horse battery"},
		{Email: "ana@example.com", Password: "short"},
	} {
		if err := s.Signup(ctx, in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Signup(%+v) = %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestLoginLockoutAndReset(t *testing.T) {
	s, clock := newTestService(t)
	ctx := context.Background()
	confirmedUser(t, s, "ana@example.com")
	for i := 0; i < maxFailedLogins; i++ {
		if _, err := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "wrong password!"}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d = %v", i, err)
		}
	}
	if _, err := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "correct horse battery"}); !errors.Is(err, ErrLocked) {
		t.Fatalf("login while locked = %v, want ErrLocked", err)
	}
	clock.advance(lockDuration + time.Second)
	if _, err := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "correct horse battery"}); err != nil {
		t.Fatalf("login after the lock = %v", err)
	}
	if _, err := s.Login(ctx, LoginInput{Email: "nobody@example.com", Password: "whatever it is"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login of an unknown address = %v, want ErrInvalidCredentials", err)
	}
}

func TestPasswordResetRevokesSessions(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	confirmedUser(t, s, "ana@example.com")
	tokens, err := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := s.ForgotPassword(ctx, "ana@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgotPassword(ctx, "nobody@example.com"); err != nil {
		t.Fatalf("ForgotPassword of an unknown address = %v, want nil", err)
	}
	reset := lastToken(t, s, "ana@example.com")
	if err := s.ResetPassword(ctx, reset, "a brand new password"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if _, err := s.Authenticate(ctx, tokens.Access); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("old session after a reset = %v, want ErrTokenInvalid", err)
	}
	if _, err := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "a brand new password"}); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
	if err := s.ResetPassword(ctx, reset, "yet another password"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("reusing a reset token = %v, want ErrTokenInvalid", err)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	s, clock := newTestService(t)
	ctx := context.Background()
	confirmedUser(t, s, "ana@example.com")
	first, err := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatal(err)
	}
	clock.advance(accessTokenTTL + time.Minute)
	if _, err := s.Authenticate(ctx, first.Access); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expired access token = %v, want ErrTokenInvalid", err)
	}
	second, err := s.Refresh(ctx, first.Refresh, "")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if second.Refresh == first.Refresh {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := s.Authenticate(ctx, second.Access); err != nil {
		t.Fatalf("new access token: %v", err)
	}
	// Presenting the rotated token again means it leaked: the session dies.
	if _, err := s.Refresh(ctx, first.Refresh, ""); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("reused refresh token = %v, want ErrTokenInvalid", err)
	}
	if _, err := s.Refresh(ctx, second.Refresh, ""); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("refresh after reuse detection = %v, want ErrTokenInvalid", err)
	}
}

func TestWorkspaceMembersAndRoles(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	ana := confirmedUser(t, s, "ana@example.com")
	bob := confirmedUser(t, s, "bob@example.com")
	carol := confirmedUser(t, s, "carol@example.com")

	team, err := s.CreateWorkspace(ctx, ana.ID, "Equipo A")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Invite(ctx, team.ID, ana.ID, "bob@example.com"); err != nil {
		t.Fatalf("Invite: %v", err)
	}
	inviteToken := lastToken(t, s, "bob@example.com")
	if _, err := s.AcceptInvitation(ctx, carol.ID, inviteToken); !errors.Is(err, ErrForbidden) {
		t.Fatalf("accepting with another account = %v, want ErrForbidden", err)
	}
	w, err := s.AcceptInvitation(ctx, bob.ID, inviteToken)
	if err != nil || w.Role != RoleMember {
		t.Fatalf("AcceptInvitation = %+v, %v", w, err)
	}
	if _, err := s.AcceptInvitation(ctx, bob.ID, inviteToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second accept = %v, want ErrTokenInvalid", err)
	}
	if _, err := s.Invite(ctx, team.ID, bob.ID, "ana@example.com"); !errors.Is(err, ErrConflict) {
		t.Fatalf("inviting a member = %v, want ErrConflict", err)
	}

	// Any member can invite, but only the owner can expel.
	if _, err := s.Invite(ctx, team.ID, bob.ID, "carol@example.com"); err != nil {
		t.Fatalf("a member inviting: %v", err)
	}
	if err := s.RemoveMember(ctx, team.ID, bob.ID, ana.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a member expelling the owner = %v, want ErrForbidden", err)
	}
	if _, err := s.AcceptInvitation(ctx, carol.ID, lastToken(t, s, "carol@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveMember(ctx, team.ID, bob.ID, carol.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a member expelling another member = %v, want ErrForbidden", err)
	}
	if err := s.RemoveMember(ctx, team.ID, ana.ID, carol.ID); err != nil {
		t.Fatalf("owner expelling a member: %v", err)
	}
	if err := s.RemoveMember(ctx, team.ID, ana.ID, ana.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("owner leaving = %v, want ErrForbidden", err)
	}
	if err := s.RemoveMember(ctx, team.ID, bob.ID, bob.ID); err != nil {
		t.Fatalf("a member leaving: %v", err)
	}
	if _, err := s.Role(ctx, team.ID, bob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("role of a former member = %v, want ErrNotFound", err)
	}
}

func TestRemovedMemberLosesWorkspaceAtOnce(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	ana := confirmedUser(t, s, "ana@example.com")
	bob := confirmedUser(t, s, "bob@example.com")
	team, _ := s.CreateWorkspace(ctx, ana.ID, "Equipo A")
	if _, err := s.Invite(ctx, team.ID, ana.ID, "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptInvitation(ctx, bob.ID, lastToken(t, s, "bob@example.com")); err != nil {
		t.Fatal(err)
	}
	tokens, err := s.Login(ctx, LoginInput{Email: "bob@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatal(err)
	}
	switched, err := s.Refresh(ctx, tokens.Refresh, team.ID)
	if err != nil {
		t.Fatalf("switching workspace: %v", err)
	}
	if p, err := s.Authenticate(ctx, switched.Access); err != nil || p.WorkspaceID != team.ID || p.Role != RoleMember {
		t.Fatalf("Authenticate in the team = %+v, %v", p, err)
	}
	if err := s.RemoveMember(ctx, team.ID, ana.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if p, err := s.Authenticate(ctx, switched.Access); err != nil || p.WorkspaceID != "" {
		t.Fatalf("Authenticate after removal = %+v, %v; want no workspace", p, err)
	}
	if _, err := s.Refresh(ctx, switched.Refresh, team.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("switching back into the team = %v, want ErrNotFound", err)
	}
}

func TestTransferAndDeleteWorkspace(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	ana := confirmedUser(t, s, "ana@example.com")
	bob := confirmedUser(t, s, "bob@example.com")
	team, _ := s.CreateWorkspace(ctx, ana.ID, "Equipo A")
	_, _ = s.Invite(ctx, team.ID, ana.ID, "bob@example.com")
	if _, err := s.AcceptInvitation(ctx, bob.ID, lastToken(t, s, "bob@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := s.TransferOwnership(ctx, team.ID, bob.ID, bob.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a member transferring = %v, want ErrForbidden", err)
	}
	if err := s.DeleteWorkspace(ctx, team.ID, bob.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a member deleting = %v, want ErrForbidden", err)
	}
	if err := s.TransferOwnership(ctx, team.ID, ana.ID, bob.ID); err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}
	if role, _ := s.Role(ctx, team.ID, ana.ID); role != RoleMember {
		t.Fatalf("old owner role = %q, want member", role)
	}
	if err := s.DeleteWorkspace(ctx, team.ID, bob.ID); err != nil {
		t.Fatalf("owner deleting: %v", err)
	}
	if _, err := s.Role(ctx, team.ID, bob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("role after delete = %v, want ErrNotFound", err)
	}
}

func TestDeleteUserAndExport(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	ana := confirmedUser(t, s, "ana@example.com")
	bob := confirmedUser(t, s, "bob@example.com")
	team, _ := s.CreateWorkspace(ctx, ana.ID, "Equipo A")
	_, _ = s.Invite(ctx, team.ID, ana.ID, "bob@example.com")
	if _, err := s.AcceptInvitation(ctx, bob.ID, lastToken(t, s, "bob@example.com")); err != nil {
		t.Fatal(err)
	}
	export, err := s.ExportUser(ctx, ana.ID)
	if err != nil || len(export.Workspaces) != 2 || export.User.Email != "ana@example.com" {
		t.Fatalf("ExportUser = %+v, %v", export, err)
	}
	if _, err := s.DeleteUser(ctx, ana.ID, "wrong password!!"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("DeleteUser with a wrong password = %v", err)
	}
	if _, err := s.DeleteUser(ctx, ana.ID, "correct horse battery"); !errors.Is(err, ErrConflict) {
		t.Fatalf("DeleteUser owning a shared workspace = %v, want ErrConflict", err)
	}
	if err := s.TransferOwnership(ctx, team.ID, ana.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	deleted, err := s.DeleteUser(ctx, ana.ID, "correct horse battery")
	if err != nil || len(deleted) != 1 {
		t.Fatalf("DeleteUser = %v, %v; want her personal workspace deleted", deleted, err)
	}
	if _, err := s.GetUser(ctx, ana.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUser after delete = %v, want ErrNotFound", err)
	}
	members, _ := s.Members(ctx, team.ID)
	if len(members) != 1 || members[0].UserID != bob.ID {
		t.Fatalf("members after the owner left = %+v", members)
	}
}

func TestAccessTokenRejectsTampering(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	confirmedUser(t, s, "ana@example.com")
	tokens, _ := s.Login(ctx, LoginInput{Email: "ana@example.com", Password: "correct horse battery"})
	body, sig, _ := strings.Cut(tokens.Access, ".")
	for _, bad := range []string{"", "x", body + ".AAAA", "AAAA." + sig, body + "." + sig + "x"} {
		if _, err := s.Authenticate(ctx, bad); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("Authenticate(%q) = %v, want ErrTokenInvalid", bad, err)
		}
	}
}

func TestAPITokenReachesOnlyItsWorkspaceAndCanBeRevoked(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	ana := confirmedUser(t, s, "ana@example.com")
	bob := confirmedUser(t, s, "bob@example.com")
	team, _ := s.CreateWorkspace(ctx, ana.ID, "Equipo A")

	if _, _, err := s.CreateAPIToken(ctx, bob.ID, team.ID, "Claude"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member creating a token = %v, want ErrNotFound", err)
	}
	if _, _, err := s.CreateAPIToken(ctx, ana.ID, team.ID, " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an unnamed token = %v, want ErrInvalidInput", err)
	}
	secret, tok, err := s.CreateAPIToken(ctx, ana.ID, team.ID, "Claude")
	if err != nil || !strings.HasPrefix(secret, "cmk_") {
		t.Fatalf("CreateAPIToken = %q, %v", secret, err)
	}
	p, err := s.Authenticate(ctx, secret)
	if err != nil || !p.APIToken || p.WorkspaceID != team.ID || p.Role != RoleOwner || p.UserID != ana.ID {
		t.Fatalf("Authenticate(api token) = %+v, %v", p, err)
	}
	if listed, _ := s.ListAPITokens(ctx, ana.ID); len(listed) != 1 || listed[0].LastUsedAt == 0 {
		t.Fatalf("ListAPITokens = %+v", listed)
	}
	if _, err := s.Authenticate(ctx, secret+"x"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a wrong token = %v, want ErrTokenInvalid", err)
	}
	if err := s.RevokeAPIToken(ctx, bob.ID, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoking someone else's token = %v, want ErrNotFound", err)
	}
	if err := s.RevokeAPIToken(ctx, ana.ID, tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, secret); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a revoked token = %v, want ErrTokenInvalid", err)
	}
}
