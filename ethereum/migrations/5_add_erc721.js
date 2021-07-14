require("dotenv").config();

const ScaleCodec = artifacts.require("ScaleCodec");
const TestToken721 = artifacts.require("TestToken721")
const ERC721App = artifacts.require("ERC721App");
const BasicInboundChannel = artifacts.require("BasicInboundChannel");
const BasicOutboundChannel = artifacts.require("BasicOutboundChannel");
const IncentivizedInboundChannel = artifacts.require("IncentivizedInboundChannel");
const IncentivizedOutboundChannel = artifacts.require("IncentivizedOutboundChannel");

module.exports = function (deployer, network, accounts) {
  deployer.then(async () => {

    if (network === 'development') {
      return
    }

    // Link libraries
    deployer.link(ScaleCodec, [ERC721App]);

    const basicInboundChannelInstance = await BasicInboundChannel.deployed()
    const basicOutboundChannelInstance = await BasicOutboundChannel.deployed()
    const incentivizedInboundChannelInstance = await IncentivizedInboundChannel.deployed()
    const incentivizedOutboundChannelInstance = await IncentivizedOutboundChannel.deployed()

    // Deploy applications
    const erc721AppInstance = await deployer.deploy(
      ERC721App,
      {
        inbound: basicInboundChannelInstance.address,
        outbound: basicOutboundChannelInstance.address,
      },
      {
        inbound: incentivizedInboundChannelInstance.address,
        outbound: incentivizedOutboundChannelInstance.address,
      },
    );
    await deployer.deploy(TestToken721, "Test Token 721", "TEST721");

    // Authorize ERC721 app to use outbound channels
    let administrator = accounts[0];
    await basicOutboundChannelInstance.authorizeDefaultOperator(
      erc721AppInstance.address, { from: administrator }
    );
    await incentivizedOutboundChannelInstance.authorizeDefaultOperator(
      erc721AppInstance.address, { from: administrator }
    );
  })
};
