package api

import (
	"github.com/gin-gonic/gin"
	"github.com/snple/beacon/http/util"
	"github.com/snple/beacon/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	as *ApiService
}

func newNodeService(as *ApiService) *NodeService {
	return &NodeService{
		as: as,
	}
}

func (s *NodeService) register(router gin.IRouter) {
	group := router.Group("/node")

	group.GET("/", s.view)
}

func (s *NodeService) view(ctx *gin.Context) {
	reply, err := s.as.Edge().GetNode().View(ctx, &pb.MyEmpty{})
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
