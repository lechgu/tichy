package auth

import "context"

type contextKey string

const userKey contextKey = "user"

// User defines auth user structure
type User struct {
	Name      string
	VectorDBs []string
}

// WithUser provides context with given user value
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFromContext provides user lookup from context
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKey).(*User)
	return u, ok
}
