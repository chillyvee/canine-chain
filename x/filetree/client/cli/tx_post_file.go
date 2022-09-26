package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	eciesgo "github.com/ecies/go/v2"
	"github.com/jackal-dao/canine/x/filetree/types"
	filetypes "github.com/jackal-dao/canine/x/filetree/types"
	"github.com/spf13/cobra"
)

var _ = strconv.Itoa(0)

func CmdPostFile() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "post-file [path] [account] [contents] [keys] [viewers] [editors]",
		Short: "post a new file to an account's file explorer",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argHashpath := args[0]
			argAccount := args[1]
			argContents := args[2]
			argKeys := args[3]
			argViewers := args[4]
			argEditors := args[5]

			viewerAddresses := strings.Split(argViewers, ",")
			editorAddresses := strings.Split(argEditors, ",")

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			//Cut out the / at the end for compatibility with types/merkle-paths.go
			trimPath := strings.TrimSuffix(argHashpath, "/")
			chunks := strings.Split(trimPath, "/")

			//Print statements left in temporarily for troubleshooting
			parentString := strings.Join(chunks[0:len(chunks)-1], "/")
			childString := string(chunks[len(chunks)-1])
			parentHash := types.MerklePath(parentString)

			h := sha256.New()
			h.Write([]byte(childString))
			childHash := fmt.Sprintf("%x", h.Sum(nil))

			//Getting the tracker from the client side safe? By the time your transaction is done, the tracker would have been incremented by many other transactions
			queryClient := filetypes.NewQueryClient(clientCtx)
			res, err := queryClient.Tracker(cmd.Context(), &filetypes.QueryGetTrackerRequest{})
			if err != nil {
				return types.ErrTrackerNotFound
			}
			trackingNumber := res.Tracker.TrackingNumber

			viewers := make(map[string]string)
			editors := make(map[string]string)

			viewerAddresses = append(viewerAddresses, clientCtx.GetFromAddress().String())
			editorAddresses = append(editorAddresses, clientCtx.GetFromAddress().String())

			for _, v := range viewerAddresses {
				if len(v) < 1 {
					continue
				}
				key, err := sdk.AccAddressFromBech32(v)
				if err != nil {
					return err
				}

				queryClient := filetypes.NewQueryClient(clientCtx)
				res, err := queryClient.Pubkey(cmd.Context(), &filetypes.QueryGetPubkeyRequest{Address: key.String()})
				if err != nil {
					return types.ErrPubKeyNotFound
				}

				pkey, err := eciesgo.NewPublicKeyFromHex(res.Pubkey.Key)
				if err != nil {
					return err
				}

				encrypted, err := clientCtx.Keyring.Encrypt(pkey.Bytes(false), []byte(argKeys))
				if err != nil {
					return err
				}

				h := sha256.New()
				h.Write([]byte(fmt.Sprintf("v%d%s", trackingNumber, v)))
				hash := h.Sum(nil)

				addressString := fmt.Sprintf("%x", hash)

				viewers[addressString] = fmt.Sprintf("%x", encrypted)
			}

			for _, v := range editorAddresses {
				if len(v) < 1 {
					continue
				}

				key, err := sdk.AccAddressFromBech32(v)
				if err != nil {
					return err
				}

				queryClient := filetypes.NewQueryClient(clientCtx)
				res, err := queryClient.Pubkey(cmd.Context(), &filetypes.QueryGetPubkeyRequest{Address: key.String()})
				if err != nil {
					return types.ErrPubKeyNotFound
				}

				pkey, err := eciesgo.NewPublicKeyFromHex(res.Pubkey.Key)
				if err != nil {
					return err
				}

				encrypted, err := clientCtx.Keyring.Encrypt(pkey.Bytes(false), []byte(argKeys))
				if err != nil {
					return err
				}

				h := sha256.New()
				h.Write([]byte(fmt.Sprintf("e%d%s", trackingNumber, v)))
				hash := h.Sum(nil)

				addressString := fmt.Sprintf("%x", hash)

				editors[addressString] = fmt.Sprintf("%x", encrypted)
			}

			jsonViewers, err := json.Marshal(viewers)
			if err != nil {
				return err
			}

			jsonEditors, err := json.Marshal(editors)
			if err != nil {
				return err
			}

			H := sha256.New()
			H.Write([]byte(fmt.Sprintf("%s", argAccount)))
			hash := H.Sum(nil)

			accountHash := fmt.Sprintf("%x", hash)

			msg := types.NewMsgPostFile(
				clientCtx.GetFromAddress().String(),
				accountHash,
				parentHash,
				childHash,
				argContents,
				string(jsonViewers),
				string(jsonEditors),
				trackingNumber,
			)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
