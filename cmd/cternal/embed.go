package main

import (
	cternal "github.com/wtnb75/cternal"
	"github.com/wtnb75/cternal/internal/api"
)

func init() {
	api.StaticFS = cternal.FrontendFS()
}
