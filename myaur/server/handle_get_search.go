package server

import (
	"github.com/haileyok/myaur/myaur/database"
	"github.com/labstack/echo/v4"
)

var (
	GetSearchInputByAllowedValues = map[string]struct{}{
		"name":         {},
		"name-desc":    {},
		"maintainer":   {},
		"depends":      {},
		"makedepends":  {},
		"optdepends":   {},
		"checkdepends": {},
	}
)

type GetSearchInput struct {
	By string `query:"by"`
}

type GetSearchOutput struct {
	Version     int                    `json:"version"`
	Type        string                 `json:"type"`
	ResultCount int                    `json:"resultcount"`
	Results     []database.PackageInfo `json:"results"`
	Error       *string                `json:"error,omitempty"`
}

func makeErrJson(error string) GetSearchOutput {
	return GetSearchOutput{
		Version:     5,
		Type:        "error",
		ResultCount: 0,
		Results:     []database.PackageInfo{},
		Error:       &error,
	}
}

// Depending on what the `by` parameter is, should receive one of the following as path:
// `name`: search by package name
// `name-desc`: search by package name and description
// `maintainer`: search by maintainer name
// `depends`: search for packages that depend on a keyword
// `makedepends`: search for packages that makedepend on a keyword
// `optdepends`: search for packages that optdepends on a keyword
// `checkdepends`: search for packages that checkdepends on a keyword
func (s *Server) handleGetSearch(e echo.Context) error {
	logger := s.logger.With("handler", "getSearch")

	var input GetSearchInput
	if err := e.Bind(&input); err != nil {
		logger.Error("failed to bind request", "err", err)
		return e.JSON(400, makeErrJson("Failed to bind request"))
	}

	logger = logger.With("input", input)

	if input.By != "" {
		if _, ok := GetSearchInputByAllowedValues[input.By]; !ok {
			logger.Error("invalid by supplied", "by", input.By)
			return e.JSON(400, makeErrJson("Invalid `by` supplied. Valid values are name, name-desc, maintainer, depends, optdepends, checkdepends"))
		}
	} else {
		input.By = "name"
	}

	term := e.Param("term")

	var pkgs []database.PackageInfo
	var err error
	switch input.By {
	case "name":
		pkgs, err = s.db.GetPackagesByName(term)
	case "name-desc":
		pkgs, err = s.db.GetPackagesByDescriptionOrName(term)
	default:
		return e.JSON(500, makeErrJson("Search method not implemented"))
		// case "maintainer":
		// case "depends":
		// case "makedepends":
		// case "optdepends":
		// case "checkdepends":
	}

	if err != nil {
		return e.JSON(500, makeErrJson("Error searching for packages"))
	}

	return e.JSON(200, GetSearchOutput{
		Version:     5,
		Type:        "search",
		ResultCount: len(pkgs),
		Results:     pkgs,
	})
}
