package main

import (
	"github.com/gogf/gf/v2/os/gctx"

	"higress-portal-backend/internal/cmd"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
