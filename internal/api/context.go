package api

import (
	"context"
)

type contextKey string

const userContextKey contextKey = "user"

type userInfo struct {
	UserID   int64
	Username string
}

func setUserContext(ctx context.Context, userID int64, username string) context.Context {
	return context.WithValue(ctx, userContextKey, &userInfo{
		UserID:   userID,
		Username: username,
	})
}

func getUserFromContext(ctx context.Context) (int64, string, bool) {
	info, ok := ctx.Value(userContextKey).(*userInfo)
	if !ok || info == nil {
		return 0, "", false
	}
	return info.UserID, info.Username, true
}
