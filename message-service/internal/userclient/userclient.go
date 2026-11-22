package userclient

import "context"

type User struct {
	ID       uint
	Email    string
	UserName string
}
type UserClient interface {
	GetUser(ctx context.Context, id uint) (User, error)
}

type TokenSource interface {
	Token(ctx context.Context) (string, error)
}
