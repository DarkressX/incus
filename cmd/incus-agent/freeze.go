package main

import (
	"errors"
	"net/http"

	"github.com/lxc/incus/v7/internal/server/response"
)

var freezeCmd = APIEndpoint{
	Name: "freeze",
	Path: "freeze",

	Post: APIEndpointAction{Handler: freezePost},
}

var unfreezeCmd = APIEndpoint{
	Name: "unfreeze",
	Path: "unfreeze",

	Post: APIEndpointAction{Handler: unfreezePost},
}

func freezePost(d *Daemon, r *http.Request) response.Response {
	if d.Features != nil && !d.Features["freeze"] {
		return response.Forbidden(errors.New("Filesystem freezing has been disabled by configuration"))
	}

	paths, err := osFreezeFilesystems()
	if err != nil {
		return response.InternalError(err)
	}

	return response.SyncResponse(true, paths)
}

func unfreezePost(d *Daemon, r *http.Request) response.Response {
	if d.Features != nil && !d.Features["freeze"] {
		return response.Forbidden(errors.New("Filesystem freezing has been disabled by configuration"))
	}

	paths, err := osUnfreezeFilesystems()
	if err != nil {
		return response.InternalError(err)
	}

	return response.SyncResponse(true, paths)
}
