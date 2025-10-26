package srcinfo

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/haileyok/myaur/myaur/database"
)

func Parse(content string) (*database.PackageInfo, error) {
	pkg := &database.PackageInfo{}
	scanner := bufio.NewScanner(strings.NewReader(content))

	// each looks like `key = val`. most of the lines will have whitespace infront of
	// them, so we remove that
	for scanner.Scan() {
		// grab the line, removing any whitespace
		line := strings.TrimSpace(scanner.Text())

		// skip any empty lines or ones that are commented out
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// we could probably split on ` = `, but i suppose its possible for these to have
		// any number of spaces infront of/after the `=`, so we'll be safe
		pts := strings.SplitN(line, "=", 2)
		if len(pts) != 2 {
			continue
		}

		// remove the extra whitespace
		key := strings.TrimSpace(pts[0])
		value := strings.TrimSpace(pts[1])

		switch key {
		case "pkgname":
			pkg.Name = value
		case "pkgbase":
			pkg.PackageBase = value
		case "pkgver":
			pkg.Version = value
		case "pkgrel":
			if pkg.Version != "" {
				pkg.Version = pkg.Version + "-" + value
			}
		case "pkgdesc":
			pkg.Description = value
		case "url":
			pkg.Url = value
		case "depends":
			pkg.Depends = append(pkg.Depends, value)
		case "makedepends":
			pkg.MakeDepends = append(pkg.MakeDepends, value)
		case "license":
			pkg.License = append(pkg.License, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning srcinfo: %w", err)
	}

	if pkg.Name == "" {
		return nil, fmt.Errorf("missing required field: pkgname")
	}

	return pkg, nil
}
