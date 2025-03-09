package api

import (
	"github.com/gin-gonic/gin"
	"github.com/snple/beacon/http/util"
	"github.com/snple/beacon/http/util/shiftime"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinService struct {
	as *ApiService
}

func newPinService(as *ApiService) *PinService {
	return &PinService{
		as: as,
	}
}

func (s *PinService) register(router gin.IRouter) {
	group := router.Group("/pin")

	group.GET("/", s.list)

	group.GET("/:id", s.getById)
	group.GET("/:id/value", s.getValueById)
	group.PATCH("/:id/value", s.setValueById)

	group.GET("/name/:name", s.getByName)
	group.POST("/name", s.getByNames)

	group.POST("/get_value", s.getValueByNames)
	group.PATCH("/set_value", s.setValueByNames)

	group.POST("/get_write", s.getWriteByNames)
	group.PATCH("/set_write", s.setWriteByNames)
}

func (s *PinService) list(ctx *gin.Context) {
	var params struct {
		util.Page `form:",inline"`
		Name      string `form:"name"`
		Tags      string `form:"tags"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	wire, err := s.as.Edge().GetWire().Name(ctx,
		&pb.Name{Name: params.Name})
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

	request := &edges.PinListRequest{
		Page:   page,
		WireId: wire.Id,
		Tags:   params.Tags,
	}

	reply, err := s.as.Edge().GetPin().List(ctx, request)
	if err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	items := reply.GetPin()

	shiftime.Pins(items)

	ctx.JSON(util.Success(gin.H{
		"wire":  wire,
		"items": items,
		"total": reply.GetCount(),
	}))
}

func (s *PinService) getById(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.as.Edge().GetPin().View(ctx, request)
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

	shiftime.Pin(reply)

	ctx.JSON(util.Success(reply))
}

func (s *PinService) getValueById(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	reply, err := s.as.Edge().GetPin().GetValue(ctx, request)
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

	shiftime.PinValue(reply)

	ctx.JSON(util.Success(reply))
}

func (s *PinService) setValueById(ctx *gin.Context) {
	request := &pb.Id{Id: ctx.Param("id")}

	var params struct {
		Value string `json:"value"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	reply, err := s.as.Edge().GetPin().SetValue(ctx,
		&pb.PinValue{Id: request.Id, Value: params.Value})
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

func (s *PinService) getByName(ctx *gin.Context) {
	name := ctx.Param("name")

	reply, err := s.as.Edge().GetPin().Name(ctx,
		&pb.Name{Name: name})
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

	shiftime.Pin(reply)

	ctx.JSON(util.Success(reply))
}

func (s *PinService) getByNames(ctx *gin.Context) {
	var params struct {
		Name []string `json:"name"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ret := make([]*pb.Pin, 0, len(params.Name))

	for _, name := range params.Name {
		reply, err := s.as.Edge().GetPin().Name(ctx,
			&pb.Name{Name: name})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.NotFound {
					continue
				}
			}

			ctx.JSON(util.Error(400, err.Error()))
			return
		}

		shiftime.Pin(reply)

		ret = append(ret, reply)
	}

	ctx.JSON(util.Success(ret))
}

func (s *PinService) getValueByNames(ctx *gin.Context) {
	var params struct {
		Name []string `json:"name"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ret := make([]*pb.PinNameValue, 0, len(params.Name))

	for _, name := range params.Name {
		reply, err := s.as.Edge().GetPin().GetValueByName(ctx,
			&pb.Name{Name: name})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.NotFound {
					continue
				}
			}

			ctx.JSON(util.Error(400, err.Error()))
			return
		}

		shiftime.PinNameValue(reply)

		ret = append(ret, reply)
	}

	ctx.JSON(util.Success(ret))
}

func (s *PinService) setValueByNames(ctx *gin.Context) {
	var params struct {
		NameValue map[string]string `json:"name_value"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	errors := make(map[string]string)

	for name, value := range params.NameValue {
		_, err := s.as.Edge().GetPin().SetValueByName(ctx,
			&pb.PinNameValue{Name: name, Value: value})
		if err != nil {
			errors[name] = err.Error()
		}
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

func (s *PinService) getWriteByNames(ctx *gin.Context) {
	var params struct {
		Name []string `json:"name"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	ret := make([]*pb.PinNameValue, 0, len(params.Name))

	for _, name := range params.Name {
		reply, err := s.as.Edge().GetPin().GetWriteByName(ctx, &pb.Name{Name: name})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.NotFound {
					continue
				}
			}
		}

		shiftime.PinNameValue(reply)

		ret = append(ret, reply)
	}

	ctx.JSON(util.Success(ret))
}

func (s *PinService) setWriteByNames(ctx *gin.Context) {
	var params struct {
		NameValue map[string]string `json:"name_value"`
	}
	if err := ctx.Bind(&params); err != nil {
		ctx.JSON(util.Error(400, err.Error()))
		return
	}

	errors := make(map[string]string)

	for name, value := range params.NameValue {
		_, err := s.as.Edge().GetPin().SetWriteByName(ctx, &pb.PinNameValue{Name: name, Value: value})
		if err != nil {
			errors[name] = err.Error()
		}
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
