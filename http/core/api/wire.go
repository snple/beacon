package api

import (
	"github.com/gin-gonic/gin"
	"github.com/snple/beacon/http/util"
	"github.com/snple/beacon/http/util/shiftime"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WireService struct {
	as *ApiService
}

func newWireService(as *ApiService) *WireService {
	return &WireService{
		as: as,
	}
}

func (s *WireService) register(router gin.IRouter) {
	group := router.Group("/wire")

	group.GET("/", s.list)

	group.GET("/:id", s.getById)

	group.GET("/name/:name", s.getByName)
	group.POST("/name", s.getByNames)

	group.PATCH("/link", s.link)
}

func (s *WireService) list(ctx *gin.Context) {
	var params struct {
		util.Page `form:",inline"`
		NodeId    string `form:"node_id"`
		Tags      string `form:"tags"`
		Source    string `form:"source"`
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

	request := &cores.WireListRequest{
		Page:   page,
		NodeId: params.NodeId,
		Tags:   params.Tags,
		Source: params.Source,
	}

	reply, err := s.as.Core().GetWire().List(ctx, request)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	items := reply.GetWire()

	shiftime.Wires(items)

	ctx.JSON(util.Success(gin.H{
		"items": items,
		"total": reply.GetCount(),
	}))
}

func (s *WireService) getById(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.as.Core().GetWire().View(ctx, request)
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

	shiftime.Wire(reply)

	ctx.JSON(util.Success(reply))
}

func (s *WireService) getByName(ctx *gin.Context) {
	name := ctx.Param("name")

	var params struct {
		NodeId string `form:"node_id"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	reply, err := s.as.Core().GetWire().Name(ctx,
		&cores.WireNameRequest{NodeId: params.NodeId, Name: name})
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

	shiftime.Wire(reply)

	ctx.JSON(util.Success(reply))
}

func (s *WireService) getByNames(ctx *gin.Context) {
	var params struct {
		NodeId string   `json:"node_id"`
		Name   []string `json:"name"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ret := make([]*pb.Wire, 0, len(params.Name))

	for _, name := range params.Name {
		reply, err := s.as.Core().GetWire().Name(ctx,
			&cores.WireNameRequest{NodeId: params.NodeId, Name: name})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.NotFound {
					continue
				}
			}

			ctx.JSON(util.Error(400, err.Error()))
			return
		}

		shiftime.Wire(reply)

		ret = append(ret, reply)
	}

	ctx.JSON(util.Success(ret))
}

func (s *WireService) link(ctx *gin.Context) {
	var params struct {
		Id     string `json:"id"`
		Status int    `json:"status"`
	}

	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	reply, err := s.as.Core().GetWire().Link(ctx,
		&cores.WireLinkRequest{Id: params.Id, Status: int32(params.Status)})
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

	ctx.JSON(util.Success(reply))
}
