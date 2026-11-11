package api

import (
	"context"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/fortressmcp"
)

// fortressTools are the tools the other apps of the fortress may call here.
// They run like the calls of a key that may start backups, and every app
// shares one budget and the cooldowns, so a cancel reaches any backup the
// fortress started.
var fortressTools = []string{"start_backup", "start_domain_backup", "start_backup_everything", "cancel_backup"}

// fortressToolsAction lets the other apps of the fortress call fortressTools,
// as far as the permission for them is on.
func (h *Handler) fortressToolsAction() fortress.Action {
	return fortressmcp.Offer(h.newOwnMCPServer(), fortressTools, func(ctx context.Context, from fortress.Caller) (context.Context, error) {
		return withMCPCaller(ctx, mcpCaller{
			Via:             viaFortress,
			Label:           from.App + " on " + from.Device,
			CanStartBackups: true,
			ClientAddr:      viaFortress,
		}), nil
	})
}

// fortressMCP brings the fortress into the MCP server: the members, their
// status and log for every key, and the tools of other apps for a key that
// may start backups here.
func (h *Handler) fortressMCP() fortressmcp.Config {
	return fortressmcp.Config{
		Fortress: func() fortressmcp.Fortress {
			if f := h.svc.fortressNow(); f != nil {
				return f
			}
			return nil
		},
		MayAct: func(ctx context.Context) bool {
			caller, _ := mcpCallerFrom(ctx)
			return caller.CanStartBackups
		},
		Called: h.logMCPCall,
	}
}
