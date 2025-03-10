package web

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/http/util"
	"github.com/snple/beacon/http/util/jwt"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/types/cache"
)

type AuthService struct {
	ws  *WebService
	jwt *jwt.GinJWTMiddleware

	cache *cache.Cache[*pb.User]
}

var identityKey = "id"

func newAuthService(ws *WebService) (*AuthService, error) {
	s := &AuthService{
		ws: ws,

		cache: cache.NewCache(func(ctx context.Context, key string) (*pb.User, time.Duration, error) {
			n, err := ws.Core().GetUser().View(ctx, &pb.Id{Id: key})
			if err != nil {
				return nil, 0, err
			}

			return n, time.Second * 3, nil
		}),
	}

	jwt, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "beacon",
		Key:         []byte(ws.dopts.jwtSecretKey),
		Timeout:     ws.dopts.jwtTimeout,
		MaxRefresh:  ws.dopts.jwtMaxRefresh,
		IdentityKey: identityKey,
		PayloadFunc: func(data any) jwt.MapClaims {
			if v, ok := data.(*pb.User); ok {
				return jwt.MapClaims{
					identityKey: v.GetId(),
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) any {
			claims := jwt.ExtractClaims(c)
			return &pb.User{
				Id: claims[identityKey].(string),
			}
		},
		Authenticator: s.authenticator,
		Authorizator:  s.authorizator,
	})
	if err != nil {
		return nil, err
	}

	s.jwt = jwt

	return s, nil
}

type login struct {
	User string `form:"user" json:"user" binding:"required"`
	Pass string `form:"pass" json:"pass" binding:"required"`
}

func (s *AuthService) register(router gin.IRouter) {
	{
		group := router.Group("/auth")

		group.POST("/login", s.jwt.LoginHandler)
	}

	group := router.Group("/auth", s.MiddlewareFunc())

	group.GET("/", s.myself)
	group.GET("/refresh_token", s.jwt.RefreshHandler)
	group.POST("/change_pass", s.changePass)
	group.POST("/force_change_pass", s.forceChangePass)
}

func (s *AuthService) authenticator(ctx *gin.Context) (any, error) {
	var loginVals login
	if err := ctx.ShouldBind(&loginVals); err != nil {
		return nil, jwt.ErrMissingLoginValues
	}

	request := &cores.LoginRequest{
		Name: loginVals.User,
		Pass: loginVals.Pass,
	}

	reply, err := s.ws.Core().GetAuth().Login(ctx, request)
	if err != nil {
		return "", err
	}

	if reply.User.GetStatus() != consts.ON {
		return "", jwt.ErrFailedAuthentication
	}

	return reply.User, nil
}

func (s *AuthService) authorizator(data any, ctx *gin.Context) bool {
	if v, ok := data.(*pb.User); ok {
		option, err := s.cache.GetWithMiss(ctx, v.GetId())
		if err != nil {
			return false
		}

		if option.IsNone() {
			return false
		}

		user := option.Unwrap()

		if user.GetStatus() != consts.ON {
			return false
		}

		ctx.Set("user_id", user.GetId())
		ctx.Set("user", user)

		return true
	}

	return false
}

func (s *AuthService) MiddlewareFunc() gin.HandlerFunc {
	return s.jwt.MiddlewareFunc()
}

func (s *AuthService) myself(ctx *gin.Context) {
	value, has := ctx.Get("user")
	if !has {
		ctx.JSON(util.Error(403, "internal error"))
		return
	}

	user := value.(*pb.User)

	type Ability struct {
		Action  string `json:"action"`
		Subject string `json:"subject"`
	}

	type Myself struct {
		*pb.User
		Ability []Ability `json:"ability"`
	}

	ablity := []Ability{
		{
			Action:  "read",
			Subject: "public",
		},
		{
			Action:  "read",
			Subject: "auth",
		},
	}

	if s.ws.GetUser().IsAdmin(user) || user.Role == "维护者" {
		ablity = append(ablity, Ability{
			Action:  "manage",
			Subject: "all",
		})
	}

	myself := Myself{user, ablity}

	ctx.JSON(util.Success(myself))
}

func (s *AuthService) changePass(ctx *gin.Context) {
	value, has := ctx.Get("user")
	if !has {
		ctx.JSON(util.Error(403, "internal error"))
		return
	}

	user := value.(*pb.User)

	var params cores.ChangePassRequest

	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	if params.GetId() != user.GetId() && user.GetName() != "root" {
		ctx.JSON(util.Error(401, "this operation requires a root user"))
		return
	}

	reply, err := s.ws.Core().GetAuth().ChangePass(ctx, &params)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ctx.JSON(util.Success(reply))
}

func (s *AuthService) forceChangePass(ctx *gin.Context) {
	value, has := ctx.Get("user")
	if !has {
		ctx.JSON(util.Error(403, "internal error"))
		return
	}

	user := value.(*pb.User)
	if user.GetName() != "root" {
		ctx.JSON(util.Error(401, "this operation requires a root user"))
		return
	}

	var params cores.ForceChangePassRequest

	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	reply, err := s.ws.Core().GetAuth().ForceChangePass(ctx, &params)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ctx.JSON(util.Success(reply))
}
