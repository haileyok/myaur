package server

import "github.com/labstack/echo/v4"

type GetInfoInput struct {
	Arg []string `query:"arg"`
}

func (s *Server) handleGetInfo(e echo.Context) error {
	logger := s.logger.With("route", "getInfo")

	// HACK: probably better way to do this...
	queryParams := e.QueryParams()
	args := queryParams["arg[]"]
	if len(args) == 0 {
		args = queryParams["arg"]
	}

	if len(args) == 0 {
		return e.JSON(400, makeErrJson("Missing `arg` parameter"))
	}

	pkgs, err := s.db.GetPackagesByNames(args)
	if err != nil {
		logger.Error("failed to lookup packages", "err", err)
		return e.JSON(500, makeErrJson("Failed to search for packages"))
	}

	return e.JSON(200, GetSearchOutput{
		Version:     5,
		Type:        "search",
		ResultCount: len(pkgs),
		Results:     pkgs,
	})
}
