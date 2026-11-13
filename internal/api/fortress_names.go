package api

import "github.com/junkerderprovinz/parleyport/kit/fortress"

// houseNames are how the house's apps write their names, for a member that
// joined through the kit's fortress command, which announces its app by the
// ID alone.
var houseNames = map[string]string{
	"arrowloop": "ArrowLoop", "bombvault": "BombVault", "cannoncommander": "CannonCommander",
	"castleparcel": "CastleParcel", "firesquire": "FireSquire", "knightloader": "KnightLoader",
	"shiplog": "ShipLog", "tollroll": "TollRoll", "towertorch": "TowerTorch",
}

// appName is the name BombVault shows for a member's app.
func appName(a fortress.App) string {
	if a.Name != "" && a.Name != a.ID {
		return a.Name
	}
	if n, ok := houseNames[a.ID]; ok {
		return n
	}
	return a.ID
}
