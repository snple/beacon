package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/snple/beacon/http/util"
	"github.com/snple/beacon/http/util/shiftime"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConstService struct {
	as *ApiService
}

func newConstService(as *ApiService) *ConstService {
	return &ConstService{
		as: as,
	}
}

func (s *ConstService) register(router gin.IRouter) {
	group := router.Group("/const")

	group.GET("/", s.list)

	group.GET("/:id", s.getById)
	group.GET("/:id/value", s.getValueById)
	group.PATCH("/:id/value", s.setValueById)

	group.GET("/name/:name", s.getByName)
	group.POST("/names", s.getByNames)

	group.POST("/get_value", s.getValueByNames)
	group.PATCH("/set_value", s.setValueByNames)
}

func (s *ConstService) list(ctx *gin.Context) {
	var params struct {
		util.Page `form:",inline"`
		NodeId    string `form:"node_id"`
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

	request := &cores.ConstListRequest{
		Page:   page,
		NodeId: params.NodeId,
		Tags:   params.Tags,
	}

	reply, err := s.as.Core().GetConst().List(ctx, request)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	items := reply.GetConst()

	shiftime.Consts(items)

	ctx.JSON(util.Success(gin.H{
		"items": items,
		"total": reply.GetCount(),
	}))
}

func (s *ConstService) getById(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.as.Core().GetConst().View(ctx, request)
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

	shiftime.Const(reply)

	ctx.JSON(util.Success(reply))
}

func (s *ConstService) getValueById(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.as.Core().GetConst().GetValue(ctx, request)
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

	shiftime.ConstValue(reply)

	ctx.JSON(util.Success(reply))
}

func (s *ConstService) setValueById(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	var params struct {
		Value string `json:"value"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	reply, err := s.as.Core().GetConst().SetValue(ctx,
		&pb.ConstValue{Id: request.Id, Value: params.Value})
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

	ctx.JSON(util.Success(gin.H{
		"ok": reply.GetBool(),
	}))
}

func (s *ConstService) getByName(ctx *gin.Context) {
	name := ctx.Param("name")

	var params struct {
		NodeId string `form:"node_id"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	reply, err := s.as.Core().GetConst().Name(ctx,
		&cores.ConstNameRequest{NodeId: params.NodeId, Name: name})
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

	shiftime.Const(reply)

	ctx.JSON(util.Success(reply))
}

func (s *ConstService) getByNames(ctx *gin.Context) {
	var params struct {
		NodeId string   `json:"node_id"`
		Names  []string `json:"names"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ret := make([]*pb.Const, 0, len(params.Names))

	for _, name := range params.Names {
		reply, err := s.as.Core().GetConst().Name(ctx,
			&cores.ConstNameRequest{NodeId: params.NodeId, Name: name})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.NotFound {
					continue
				}
			}

			ctx.JSON(util.Error(400, err.Error()))
			return
		}

		shiftime.Const(reply)

		ret = append(ret, reply)
	}

	ctx.JSON(util.Success(ret))
}

func (s *ConstService) getValueByNames(ctx *gin.Context) {
	var params struct {
		NodeId string   `json:"node_id"`
		Names  []string `json:"names"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ret := make([]*cores.ConstNameValue, 0, len(params.Names))

	for _, name := range params.Names {
		reply, err := s.as.Core().GetConst().GetValueByName(ctx,
			&cores.ConstGetValueByNameRequest{NodeId: params.NodeId, Name: name})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.NotFound {
					continue
				}
			}

			ctx.JSON(util.Error(400, err.Error()))
			return
		}

		shiftTimeForConstNameValue(reply)

		ret = append(ret, reply)
	}

	ctx.JSON(util.Success(ret))
}

func (s *ConstService) setValueByNames(ctx *gin.Context) {
	var params struct {
		NodeId     string            `json:"node_id"`
		NameValues map[string]string `json:"values"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	errors := make(map[string]string)

	for name, value := range params.NameValues {
		_, err := s.as.Core().GetConst().SetValueByName(ctx,
			&cores.ConstNameValue{NodeId: params.NodeId, Name: name, Value: value})
		if err != nil {
			errors[name] = err.Error()
		}
		time.Sleep(time.Millisecond)
	}

	if len(errors) > 0 {
		ctx.JSON(util.Success(gin.H{
			"ok":     false,
			"errors": errors,
		}))

		return
	}

	ctx.JSON(util.Success(gin.H{
		"ok": true,
	}))
}

func shiftTimeForConstNameValue(item *cores.ConstNameValue) {
	if item != nil {
		item.Updated = item.Updated / 1000
	}
}
