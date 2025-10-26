package server

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/labstack/echo/v4"
)

func (s *Server) handleGit(e echo.Context) error {
	if s.repoPath == "" {
		return e.String(503, "Git operations require local mirror")
	}

	// strip off any leading slash from the path
	path := e.Request().URL.Path
	if after, ok := strings.CutPrefix(path, "/"); ok {
		path = after
	}

	// get the name and git path from the url path
	pts := strings.SplitN(path, "/", 2)
	if len(pts) == 0 {
		return e.String(404, "Not Found")
	}

	packageName := strings.TrimSuffix(pts[0], ".git")

	var gitPath string
	if len(pts) > 1 {
		gitPath = pts[1]
	}

	// return a symref pointing to master for head requests
	if gitPath == "HEAD" {
		e.Response().Header().Set("Content-Type", "text/plain")
		return e.String(200, "ref: refs/heads/master\n")
	}

	// handle info/refs request
	if gitPath == "info/refs" && strings.Contains(e.Request().URL.RawQuery, "service=git-upload-pack") {
		return s.serveInfoRefs(e, packageName)
	}

	// handle git-upload-pack request
	if gitPath == "git-upload-pack" {
		return s.serveUploadPack(e, packageName)
	}

	return e.String(404, "Not Found")
}

func (s *Server) serveInfoRefs(e echo.Context, packageName string) error {
	logger := s.logger.With("route", "handleGit", "git-component", "serveInfoRefs", "package-name", packageName)

	cmd := exec.Command("git", "-C", s.repoPath, "show-ref", fmt.Sprintf("refs/heads/%s", packageName))
	output, err := cmd.Output()
	if err != nil {
		logger.Error("branch not found", "err", err)
		return e.String(404, "Package not found")
	}

	refLine := strings.TrimSpace(string(output))
	refParts := strings.Fields(refLine)
	if len(refParts) != 2 {
		logger.Error("invalid ref format", "output", refLine)
		return e.String(500, "Invalid ref format")
	}

	commitHash := refParts[0]

	// WARNING: SLOP CODE
	// claude apparently knows how to create these smart HTPP responses for git. it works on my machine,
	// but...lol
	var buf bytes.Buffer

	// Write packet: service announcement
	service := "# service=git-upload-pack\n"
	buf.WriteString(fmt.Sprintf("%04x%s", len(service)+4, service))
	buf.WriteString("0000") // flush packet

	// Advertise HEAD and master branch
	// Format: hash + SP + ref + NULL + capabilities + LF
	// NOTE: these were the ones claude kept adding until yay didn't yell at me anymore. not sure if they are all needed though
	capabilities := "multi_ack multi_ack_detailed thin-pack side-band side-band-64k ofs-delta shallow no-progress include-tag symref=HEAD:refs/heads/master"

	// Advertise HEAD first with symref capability
	headRef := fmt.Sprintf("%s HEAD\x00%s\n", commitHash, capabilities)
	buf.WriteString(fmt.Sprintf("%04x%s", len(headRef)+4, headRef))

	// because aur usually has a single repo for each package, but we have a single repo with individual
	// branches for each package, we need to spoof this to show that the ref is refs/heads/master
	masterRef := fmt.Sprintf("%s refs/heads/master\n", commitHash)
	buf.WriteString(fmt.Sprintf("%04x%s", len(masterRef)+4, masterRef))

	// claude says this is the flush packet
	buf.WriteString("0000")

	e.Response().Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
	e.Response().Header().Set("Cache-Control", "no-cache")
	return e.Blob(200, "application/x-git-upload-pack-advertisement", buf.Bytes())
}

func (s *Server) serveUploadPack(e echo.Context, packageName string) error {
	logger := s.logger.With("route", "handleGit", "git-component", "serveUploadPack", "package-name", packageName)

	bodyBytes, err := io.ReadAll(e.Request().Body)
	if err != nil {
		logger.Error("failed to read upload-pack request", "err", err)
		return e.String(400, "Failed to read request")
	}

	// because aur usually has a single repo for each package, but we have a single repo with individual
	// branches for each package, we need to spoof this to show that the ref is refs/heads/master
	modifiedBody := bytes.ReplaceAll(bodyBytes, []byte("refs/heads/master"), fmt.Appendf(nil, "refs/heads/%s", packageName))

	// then we can use upload-pack to serve the pack file
	cmd := exec.Command("git", "upload-pack", "--stateless-rpc", s.repoPath)
	cmd.Stdin = bytes.NewReader(modifiedBody)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Error("upload-pack failed", "err", err, "stderr", stderr.String(), "package", packageName)
		return e.String(500, fmt.Sprintf("upload pack failed: %s", stderr.String()))
	}

	e.Response().Header().Set("Content-Type", "application/x-git-upload-pack-result")
	e.Response().Header().Set("Cache-Control", "no-cache")
	return e.Blob(200, "application/x-git-upload-pack-result", stdout.Bytes())
}
