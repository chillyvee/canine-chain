package keeper

import (
	"context"
	"encoding/json"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/jackalLabs/canine-chain/v3/x/filetree/types"
)

func (k msgServer) RemoveEditors(goCtx context.Context, msg *types.MsgRemoveEditors) (*types.MsgRemoveEditorsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	file, found := k.GetFiles(ctx, msg.Address, msg.FileOwner)
	if !found {
		return nil, types.ErrFileNotFound
	}

	isOwner := IsOwner(file, msg.Creator)

	if !isOwner {
		return nil, types.ErrNotOwner
	}

	// Continue
	peacc := file.EditAccess

	jeacc := make(map[string]string)
	if err := json.Unmarshal([]byte(peacc), &jeacc); err != nil {
		return nil, types.ErrCantUnmarshall
	}

	ids := strings.Split(msg.EditorIds, ",")
	for _, v := range ids {
		delete(jeacc, v)
	}

	eaccbytes, err := json.Marshal(jeacc)
	if err != nil {
		return nil, types.ErrCantMarshall
	}
	newEditors := string(eaccbytes)

	file.EditAccess = newEditors

	k.SetFiles(ctx, file)

	return &types.MsgRemoveEditorsResponse{}, nil
}
