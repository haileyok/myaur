package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (s *Server) handleRpc(e echo.Context) error {
	queryParams := e.QueryParams()
	rpcType := e.QueryParam("type")
	version := e.QueryParam("v")

	if version == "" {
		version = "5"
	}

	// HACK: there's probably a better way to do this with echo...
	args := queryParams["arg[]"]
	if len(args) == 0 {
		args = queryParams["arg"]
	}

	switch rpcType {
	case "search":
		// there will be a single arg in search, since it does a `like` match
		if len(args) == 0 {
			return e.JSON(http.StatusBadRequest, makeErrJson("Missing `arg` parameter"))
		}
		e.SetPath("/rpc/v5/search/:term")
		e.SetParamNames("term")
		e.SetParamValues(args[0])
		return s.handleGetSearch(e)
	case "info", "query":
		// there whould be an array, i.e. arg[]=discord-canary&arg[]=slack
		if len(args) == 0 {
			return e.JSON(http.StatusBadRequest, makeErrJson("Missing `arg` parameter"))
		}
		e.SetPath("/rpc/v5/info")
		return s.handleGetInfo(e)
	default:
		return e.JSON(http.StatusBadRequest, makeErrJson("Missing or invalid `type` parameter"))
	}
}
