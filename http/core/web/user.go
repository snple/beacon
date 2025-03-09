package web

import (
	"github.com/gin-gonic/gin"
	"github.com/snple/beacon/http/util"
	"github.com/snple/beacon/http/util/shiftime"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserService struct {
	ws *WebService
}

func newUserService(ws *WebService) *UserService {
	return &UserService{
		ws: ws,
	}
}

func (s *UserService) register(router gin.IRouter) {
	group := router.Group("/user")

	group.Use(s.ws.GetAuth().MiddlewareFunc())

	group.GET("/", s.list)
	group.GET("/:id", s.get)
	group.POST("/", s.post)
	group.PATCH("/:id", s.patch)
	group.PATCH("/:id/status", s.status)
	group.DELETE("/:id", s.delete)
}

func (s *UserService) list(ctx *gin.Context) {
	var params struct {
		util.Page `form:",inline"`
		Tags      string `form:"tags"`
	}

	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	page := &pb.Page{
		Limit:   params.Limit,
		Offset:  params.Offset,
		Search:  params.Search,
		OrderBy: params.OrderBy,
		Sort:    pb.Page_ASC,
	}

	if params.Sort > 0 {
		page.Sort = pb.Page_DESC
	}

	request := &cores.UserListRequest{
		Page: page,
		Tags: params.Tags,
	}

	reply, err := s.ws.Core().GetUser().List(ctx, request)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	items := reply.GetUser()

	shiftime.Users(items)

	ctx.JSON(util.Success(gin.H{
		"items": items,
		"total": reply.GetCount(),
	}))
}

func (s *UserService) get(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.ws.Core().GetUser().View(ctx, request)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				ctx.JSON(util.Error(404, err.Error()))
				return
			}
		}

		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	shiftime.User(reply)

	ctx.JSON(util.Success(reply))
}

func (s *UserService) post(ctx *gin.Context) {
	value, has := ctx.Get("user")
	if !has {
		ctx.JSON(util.Error(403, "internal error"))
		return
	}

	user := value.(*pb.User)
	if !s.IsAdmin(user) {
		ctx.JSON(util.Error(401, "this operation requires a admin user"))
		return
	}

	var params pb.User

	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	params.Id = ""

	reply, err := s.ws.Core().GetUser().Create(ctx, &params)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	shiftime.User(reply)

	ctx.JSON(util.Success(reply))
}

func (s *UserService) patch(ctx *gin.Context) {
	value, has := ctx.Get("user")
	if !has {
		ctx.JSON(util.Error(403, "internal error"))
		return
	}

	user := value.(*pb.User)

	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.ws.Core().GetUser().View(ctx, request)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	var params pb.User

	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	{
		if reply.GetId() != user.GetId() && !s.IsAdmin(user) {
			ctx.JSON(util.Error(401, "this operation requires a admin user"))
			return
		}

		if reply.GetStatus() != params.GetStatus() && !s.IsAdmin(user) {
			ctx.JSON(util.Error(401, "this operation requires a admin user"))
			return
		}

		if reply.GetRole() != params.GetRole() && !s.IsAdmin(user) {
			ctx.JSON(util.Error(401, "this operation requires a admin user"))
			return
		}
	}

	reply.Name = params.Name
	reply.Desc = params.Desc
	reply.Tags = params.Tags
	reply.Role = params.Role
	reply.Status = params.Status

	reply2, err := s.ws.Core().GetUser().Update(ctx, reply)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ctx.JSON(util.Success(gin.H{"id": reply2.GetId()}))
}

func (s *UserService) delete(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	_, err := s.ws.Core().GetUser().Delete(ctx, request)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ctx.JSON(util.Success(gin.H{"id": ctx.Param("id")}))
}

func (s *UserService) IsAdmin(user *pb.User) bool {
	return user.GetName() == "root" || user.GetRole() == "admin" || user.GetRole() == "管理员"
}

func (s *UserService) status(ctx *gin.Context) {
	value, has := ctx.Get("user")
	if !has {
		ctx.JSON(util.Error(403, "internal error"))
		return
	}

	user := value.(*pb.User)

	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.ws.Core().GetUser().View(ctx, request)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	var params struct {
		Status int32 `json:"status"`
	}

	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	if reply.GetId() != user.GetId() && !s.IsAdmin(user) {
		ctx.JSON(util.Error(401, "this operation requires a admin user"))
		return
	}

	reply.Status = params.Status

	reply2, err := s.ws.Core().GetUser().Update(ctx, reply)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ctx.JSON(util.Success(gin.H{"id": reply2.GetId()}))
}
