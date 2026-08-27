package controlplane

import "context"

func withUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func userFromContext(ctx context.Context) User {
	user, _ := ctx.Value(userContextKey{}).(User)
	return user
}
