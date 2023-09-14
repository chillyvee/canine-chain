package gmp_testing

import (
	"encoding/json"
	"fmt"
	"testing"

	ibctesting "github.com/cosmos/ibc-go/v4/testing"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	gmpmiddlewaretypes "github.com/jackalLabs/canine-chain/v3/gmpmiddleware/types"
)

type GMPTestSuite struct {
	GMPTestHelper

	coordinator *ibctesting.Coordinator

	chainA TestChain
	chainB TestChain

	pathAB *ibctesting.Path
	pathBA *ibctesting.Path
}

func TestGMPTestSuite(t *testing.T) {
	suite.Run(t, new(GMPTestSuite))
}

func (suite *GMPTestSuite) SetupTest() {
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

func (suite *GMPTestSuite) TestOnRecvPacket() {
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

	axelargeneralmsg := gmpmiddlewaretypes.Message{
		SourceChain:   "placeholderSourceChain",
		SourceAddress: "placeholderSourceAddress",
		Payload:       []byte("placeholder"),
		Type:          1,
	}

	bz, err := json.Marshal(axelargeneralmsg)
	suite.Require().NoError(err) // message committed

	data := transfertypes.NewFungibleTokenPacketData(trace.GetFullDenomPath(), amount.String(), suite.chainA.SenderAccount.GetAddress().String(), receiver)
	data.Memo = string(bz)
	packet := channeltypes.NewPacket(data.GetBytes(), seq, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, clienttypes.NewHeight(1, 100), 0)

	// we expect a returned acknowledgement
	ack := suite.chainB.GetJackalApp().GmpMiddlewareStack.OnRecvPacket(suite.chainB.GetContext(), packet, suite.chainA.SenderAccount.GetAddress())

	suite.Require().True(ack.Success())
}

// TO DO: unmarshal the acknowledgement. Not sure why it can't be unmarshalled at this time.
func (suite *GMPTestSuite) TestRecvTransferWithMetadata() {
	// Setup contract
	suite.chainA.StoreContractCode(&suite.Suite, "./bytecode/echo.wasm")
	addr := suite.chainA.InstantiateContract(&suite.Suite, "{}", 1)
	fmt.Println(addr)
	ackBytes := suite.receivePacket(addr.String(), fmt.Sprintf(`{"wasm": {"contract": "%s", "msg": {"echo": {"msg": "test"} } } }`, addr))
	fmt.Println(ackBytes)
	ackStr := string(ackBytes)
	fmt.Println(ackStr)
	// from osmosis:
	// var ack map[string]string // This can't be unmarshalled to Acknowledgement because it's fetched from the events
	// err := json.Unmarshal(ackBytes, &ack)
}

func (suite *GMPTestSuite) receivePacket(receiver, memo string) []byte {
	return suite.receivePacketWithSequence(receiver, memo, 0)
}

func (suite *GMPTestSuite) receivePacketWithSequence(receiver, memo string, prevSequence uint64) []byte {
	channelCap := suite.chainB.GetChannelCapability(
		suite.pathAB.EndpointB.ChannelConfig.PortID,
		suite.pathAB.EndpointB.ChannelID)
	fmt.Println(channelCap)

	packet := suite.makeMockPacket(receiver, memo, prevSequence)

	app := suite.chainB.GetJackalApp()
	ibcKeeper := app.GetIBCKeeper()
	scopedIBCKeeper := app.GetScopedIBCKeeper()

	err := suite.chainB.GetJackalApp().GmpMiddlewareStack.SendPacket(scopedIBCKeeper, *ibcKeeper,
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

// NOTE: Always make sure this resembles osmosis' mock packet
func (suite *GMPTestSuite) makeMockPacket(receiver, memo string, prevSequence uint64) channeltypes.Packet {
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

func (suite *GMPTestSuite) TestPacketsThatShouldBeSkipped() {
	var sequence uint64
	receiver := suite.chainB.SenderAccount.GetAddress().String()

	testCases := []struct {
		memo           string
		expPassthrough bool
	}{
		{"", true},
		{"{01]", true}, // bad json
		{"{}", true},
		{`{"something": ""}`, true},
		{`{"wasm": "test"}`, false},
		{`{"wasm": []`, true}, // invalid top level JSON - so shouldn't this be FALSE?...because it couldn't pass through?
		{`{"wasm": {}`, true}, // invalid top level JSON
		// {`{"wasm": []}`, false}, // shouldn't this be TRUE because it passed through?
		// {`{"wasm": {}}`, false},
		// {`{"wasm": {"contract": "something"}}`, false},
		// {`{"wasm": {"contract": "osmo1clpqr4nrk4khgkxj78fcwwh6dl3uw4epasmvnj"}}`, false},
		{`{"wasm": {"msg": "something"}}`, false},
		// // invalid receiver
		// {`{"wasm": {"contract": "osmo1clpqr4nrk4khgkxj78fcwwh6dl3uw4epasmvnj", "msg": {}}}`, false},
		// // msg not an object
		// {fmt.Sprintf(`{"wasm": {"contract": "%s", "msg": 1}}`, receiver), false},
	}

	for _, tc := range testCases {
		ackBytes := suite.receivePacketWithSequence(receiver, tc.memo, sequence)
		ackStr := string(ackBytes)
		fmt.Println(ackStr)
		var ack map[string]string // This can't be unmarshalled to Acknowledgement because it's fetched from the events
		err := json.Unmarshal(ackBytes, &ack)
		suite.Require().NoError(err)
		if tc.expPassthrough {
			suite.Require().Equal("AQ==", ack["result"], tc.memo)
		} else {
			fmt.Printf("The ackStr is %s", ackStr)
			suite.Require().Contains(ackStr, "error", tc.memo)
		}
		sequence += 1
	}
}
