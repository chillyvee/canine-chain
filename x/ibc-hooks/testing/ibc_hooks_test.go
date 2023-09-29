package testing

import (
	"encoding/json"
	"fmt"
	"testing"

	ibctesting "github.com/cosmos/ibc-go/v4/testing"
	"github.com/jackalLabs/canine-chain/v3/testutil"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
)

type IBCHooksTestSuite struct {
	TestHelper

	coordinator *ibctesting.Coordinator

	chainA TestChain
	chainB TestChain

	pathAB *ibctesting.Path
	pathBA *ibctesting.Path
}

func TestIBCHooksTestSuite(t *testing.T) {
	suite.Run(t, new(IBCHooksTestSuite))
}

func (suite *IBCHooksTestSuite) SetupTest() {
	suite.Setup(suite.T())

	ibctesting.DefaultTestingAppInit = SetupTestingApp

	suite.coordinator = ibctesting.NewCoordinator(suite.T(), 2)

	suite.chainA = TestChain{
		TestChain: suite.coordinator.GetChain(ibctesting.GetChainID(1)),
	}

	suite.chainB = TestChain{
		TestChain: suite.coordinator.GetChain(ibctesting.GetChainID(2)),
	}

	suite.pathAB = NewTransferPath(suite.chainA, suite.chainB)
	suite.coordinator.Setup(suite.pathAB)

	suite.pathBA = NewTransferPath(suite.chainB, suite.chainA)
	suite.coordinator.Setup(suite.pathBA)
}

func NewTransferPath(chainA, chainB TestChain) *ibctesting.Path {
	path := ibctesting.NewPath(chainA.TestChain, chainB.TestChain)
	path.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	path.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	path.EndpointA.ChannelConfig.Version = transfertypes.Version
	path.EndpointB.ChannelConfig.Version = transfertypes.Version

	return path
}

func (suite *IBCHooksTestSuite) TestOnRecvPacket() {
	var (
		trace    transfertypes.DenomTrace
		amount   sdk.Int
		receiver string
		//  status   testutils.Status don't think we need for now
	)

	// need this later

	suite.SetupTest() // reset

	path := NewTransferPath(suite.chainA, suite.chainB)
	suite.coordinator.Setup(path)
	receiver = suite.chainB.SenderAccount.GetAddress().String() // looks like this is auto generated
	// status = testutils.Status{} don't think we need a status for now

	amount = sdk.NewInt(100)
	seq := uint64(1)

	trace = transfertypes.ParseDenomTrace(sdk.DefaultBondDenom)

	// do we need to send coins first?
	// send coin from chainA to chainB
	transferMsg := transfertypes.NewMsgTransfer(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, sdk.NewCoin(trace.IBCDenom(), amount), suite.chainA.SenderAccount.GetAddress().String(), receiver, clienttypes.NewHeight(1, 110), 0)
	_, err := suite.chainA.SendMsgs(transferMsg)
	suite.Require().NoError(err) // message committed

	genericMessage := "placeholder"

	bz, err := json.Marshal(genericMessage)
	suite.Require().NoError(err) // message committed

	data := transfertypes.NewFungibleTokenPacketData(trace.GetFullDenomPath(), amount.String(), suite.chainA.SenderAccount.GetAddress().String(), receiver)
	data.Memo = string(bz)
	packet := channeltypes.NewPacket(data.GetBytes(), seq, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, clienttypes.NewHeight(1, 100), 0)

	// we expect a returned acknowledgement
	ack := suite.chainB.GetJackalApp().GetIBCStack().OnRecvPacket(suite.chainB.GetContext(), packet, suite.chainA.SenderAccount.GetAddress())

	suite.Require().True(ack.Success())
}

// NOTE: Always make sure this resembles osmosis' mock packet
func (suite *IBCHooksTestSuite) makeMockPacket(receiver, memo string, prevSequence uint64) channeltypes.Packet {
	packetData := transfertypes.FungibleTokenPacketData{
		Denom:    sdk.DefaultBondDenom,
		Amount:   "1",
		Sender:   suite.chainB.SenderAccount.GetAddress().String(),
		Receiver: receiver,
		Memo:     memo, // attempted removing memo but packet still won't send. Nil pointer de-reference error remains the same.
	}

	return channeltypes.NewPacket(
		packetData.GetBytes(),
		prevSequence+1,
		suite.pathAB.EndpointB.ChannelConfig.PortID,
		suite.pathAB.EndpointB.ChannelID,
		suite.pathAB.EndpointA.ChannelConfig.PortID,
		suite.pathAB.EndpointA.ChannelID,
		clienttypes.NewHeight(0, 100),
		0,
	)
}

func (suite *IBCHooksTestSuite) receivePacket(receiver, memo string) []byte {
	return suite.receivePacketWithSequence(receiver, memo, 0)
}

func (suite *IBCHooksTestSuite) receivePacketWithSequence(receiver, memo string, prevSequence uint64) []byte {
	channelCap := suite.chainB.GetChannelCapability(
		suite.pathAB.EndpointB.ChannelConfig.PortID,
		suite.pathAB.EndpointB.ChannelID)

	packet := suite.makeMockPacket(receiver, memo, prevSequence)

	err := suite.chainB.GetJackalApp().HooksICS4Wrapper.SendPacket(
		suite.chainB.GetContext(), channelCap, packet)
	suite.Require().NoError(err, "IBC send failed. Expected success. %s", err)

	// Update both clients
	err = suite.pathAB.EndpointB.UpdateClient()
	suite.Require().NoError(err)
	err = suite.pathAB.EndpointA.UpdateClient()
	suite.Require().NoError(err)

	// recv in chain a
	res, err := suite.pathAB.EndpointA.RecvPacketWithResult(packet)
	suite.Require().NoError(err)

	// get the ack from the chain a's response
	ack, err := ibctesting.ParseAckFromEvents(res.GetEvents())
	suite.Require().NoError(err)

	// manually send the acknowledgement to chain b
	err = suite.pathAB.EndpointA.AcknowledgePacket(packet, ack)
	suite.Require().NoError(err)
	return ack
}

// // TO DO: unmarshal the acknowledgement. Not sure why it can't be unmarshalled at this time.
func (suite *IBCHooksTestSuite) TestRecvTransferWithMetadata() {
	logger, logFile := testutil.CreateLogger()
	// Setup contract
	suite.chainA.StoreContractCode(&suite.Suite, "./bytecode/echo.wasm")
	addr := suite.chainA.InstantiateContract(&suite.Suite, "{}", 1)
	logger.Printf("The contract address is %s:\n", addr)
	ackBytes := suite.receivePacket(addr.String(), fmt.Sprintf(`{"wasm": {"contract": "%s", "msg": {"echo": {"msg": "test"} } } }`, addr))
	logger.Printf("Acknowledgemenet bytes is %s:\n", ackBytes)
	ackStr := string(ackBytes)
	logger.Printf("Acknowledgemenet string is %s:\n", ackBytes)

	fmt.Println(ackStr)

	var ack map[string]string // This can't be unmarshalled to Acknowledgement because it's fetched from the events
	err := json.Unmarshal(ackBytes, &ack)
	fmt.Println(err)
	suite.Require().NoError(err)
	logFile.Close()
	suite.Require().NotContains(ack, "error")
	// TO DO: Implement our own acknowledgement string
	// suite.Require().Equal(ack["result"], "eyJjb250cmFjdF9yZXN1bHQiOiJkR2hwY3lCemFHOTFiR1FnWldOb2J3PT0iLCJpYmNfYWNrIjoiZXlKeVpYTjFiSFFpT2lKQlVUMDlJbjA9In0=")
}
