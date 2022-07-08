package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/themarstonconnell/telescope/x/telescope/keeper"
	"github.com/themarstonconnell/telescope/x/telescope/types"
)

func SimulateMsgList(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgList{
			Creator: simAccount.Address.String(),
		}

		// TODO: Handling the List simulation

		return simtypes.NoOpMsg(types.ModuleName, msg.Type(), "List simulation not implemented"), nil, nil
	}
}
