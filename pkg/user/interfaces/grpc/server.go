/*
Package grpc provides user grpc server
*/
package grpc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/vardius/go-api-boilerplate/pkg/common/application/errors"
	"github.com/vardius/go-api-boilerplate/pkg/common/application/jwt"
	"github.com/vardius/go-api-boilerplate/pkg/common/infrastructure/commandbus"
	"github.com/vardius/go-api-boilerplate/pkg/common/infrastructure/eventbus"
	"github.com/vardius/go-api-boilerplate/pkg/common/infrastructure/eventstore"
	"github.com/vardius/go-api-boilerplate/pkg/user/application"
	"github.com/vardius/go-api-boilerplate/pkg/user/domain/user"
	"github.com/vardius/go-api-boilerplate/pkg/user/infrastructure/persistence/mysql"
	"github.com/vardius/go-api-boilerplate/pkg/user/infrastructure/proto"
	"github.com/vardius/go-api-boilerplate/pkg/user/infrastructure/repository"
)

type userServer struct {
	commandBus commandbus.CommandBus
	eventBus   eventbus.EventBus
	eventStore eventstore.EventStore
	jwt        jwt.Jwt
}

// NewServer returns new user server object
func NewServer(cb commandbus.CommandBus, eb eventbus.EventBus, es eventstore.EventStore, db *sql.DB, j jwt.Jwt) proto.UserServer {
	s := &userServer{cb, eb, es, j}

	userRepository := repository.NewUserRepository(es, eb)
	userMYSQLRepository := mysql.NewUserRepository(db)

	registerCommandHandlers(cb, userRepository)
	registerEventHandlers(eb, db, userMYSQLRepository)

	return s
}

// DispatchCommand implements proto.UserServer interface
func (s *userServer) DispatchCommand(ctx context.Context, cmd *proto.DispatchCommandRequest) (*empty.Empty, error) {
	out := make(chan error)
	defer close(out)

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				out <- errors.Newf(errors.INTERNAL, "Recovered in f %v", rec)
			}
		}()

		c, err := buildDomainCommand(ctx, cmd)
		if err != nil {
			out <- err
			return
		}

		s.commandBus.Publish(ctx, fmt.Sprintf("%T", c), c, out)
	}()

	select {
	case <-ctx.Done():
		return new(empty.Empty), ctx.Err()
	case err := <-out:
		return new(empty.Empty), err
	}
}

func registerCommandHandlers(cb commandbus.CommandBus, r user.Repository) {
	cb.Subscribe(fmt.Sprintf("%T", &user.RegisterWithEmail{}), user.OnRegisterWithEmail(r))
	cb.Subscribe(fmt.Sprintf("%T", &user.RegisterWithGoogle{}), user.OnRegisterWithGoogle(r))
	cb.Subscribe(fmt.Sprintf("%T", &user.RegisterWithFacebook{}), user.OnRegisterWithFacebook(r))
	cb.Subscribe(fmt.Sprintf("%T", &user.ChangeEmailAddress{}), user.OnChangeEmailAddress(r))
}

func registerEventHandlers(eb eventbus.EventBus, db *sql.DB, r mysql.UserRepository) {
	eb.Subscribe(fmt.Sprintf("%T", &user.WasRegisteredWithEmail{}), application.WhenUserWasRegisteredWithEmail(db, r))
	eb.Subscribe(fmt.Sprintf("%T", &user.WasRegisteredWithGoogle{}), application.WhenUserWasRegisteredWithGoogle(db, r))
	eb.Subscribe(fmt.Sprintf("%T", &user.WasRegisteredWithFacebook{}), application.WhenUserWasRegisteredWithFacebook(db, r))
	eb.Subscribe(fmt.Sprintf("%T", &user.EmailAddressWasChanged{}), application.WhenUserEmailAddressWasChanged(db, r))
}
