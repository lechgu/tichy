package auth

import "context"

type contextKey string

const userKey contextKey = "user"

type User struct {
	Name       string
	Collection string
}

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKey).(*User)
	return u, ok
}
