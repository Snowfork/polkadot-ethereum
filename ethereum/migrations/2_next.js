require("dotenv").config();

const ScaleCodec = artifacts.require("ScaleCodec");
const ETHApp = artifacts.require("ETHApp");
const ERC20App = artifacts.require("ERC20App");
const DOTApp = artifacts.require("DOTApp");
const TestToken = artifacts.require("TestToken");
const ValidatorRegistry = artifacts.require("ValidatorRegistry");
const MMRVerification = artifacts.require("MMRVerification");
const MerkleProof = artifacts.require("MerkleProof");
const Bitfield = artifacts.require("Bitfield");
const Blake2b = artifacts.require("Blake2b");
const BasicInboundChannel = artifacts.require("BasicInboundChannel");
const IncentivizedInboundChannel = artifacts.require("IncentivizedInboundChannel");
const ParachainLightClient = artifacts.require("ParachainLightClient");

const channels = {
  basic: {
    inbound: {
      contract: BasicInboundChannel,
      instance: null
    },
    outbound: {
      contract: artifacts.require("BasicOutboundChannel"),
      instance: null,
    }
  },
  incentivized: {
    inbound: {
      contract: IncentivizedInboundChannel,
      instance: null
    },
    outbound: {
      contract: artifacts.require("IncentivizedOutboundChannel"),
      instance: null
    }
  },
}

const contracts = {
  beefylightclient: {
    contract: artifacts.require("BeefyLightClient"),
    instance: null
  },
  blake2b: {
    contract: artifacts.require("Blake2b"),
    instance: null
  }
}

module.exports = function (deployer, network, accounts) {
  deployer.then(async () => {

    if (network === 'development') {
      return
    }

    // Deploy & link libraries
    await deployer.deploy(ScaleCodec);
    await deployer.deploy(MerkleProof);
    await deployer.deploy(Bitfield);
    deployer.link(Bitfield, [contracts.beefylightclient.contract]);
    deployer.link(ScaleCodec, [ETHApp, ERC20App, DOTApp, contracts.beefylightclient.contract, ParachainLightClient, BasicInboundChannel, IncentivizedInboundChannel]);
    deployer.link(MerkleProof, [ValidatorRegistry, ParachainLightClient, BasicInboundChannel, IncentivizedInboundChannel]);
    await deployer.deploy(ParachainLightClient);
    deployer.link(ParachainLightClient, [BasicInboundChannel, IncentivizedInboundChannel]);

    // TODO: Hardcoded for testing
    const root = "0x697ea2a8fe5b03468548a7a413424a6292ab44a82a6f5cc594c3fa7dda7ce402";
    const numValidators = 2;
    const valRegistry = await deployer.deploy(ValidatorRegistry, root, numValidators, 0);
    const mmrVerification = await deployer.deploy(MMRVerification);
    const blake2b = await deployer.deploy(Blake2b);

    contracts.beefylightclient.instance = await deployer.deploy(
      contracts.beefylightclient.contract,
      valRegistry.address,
      mmrVerification.address,
      blake2b.address,
      0
    );

    await valRegistry.transferOwnership(contracts.beefylightclient.instance.address)

    channels.basic.inbound.instance = await deployer.deploy(
      channels.basic.inbound.contract,
      contracts.beefylightclient.instance.address
    );
    channels.basic.outbound.instance = await deployer.deploy(channels.basic.outbound.contract);
    channels.incentivized.inbound.instance = await deployer.deploy(
      channels.incentivized.inbound.contract,
      contracts.beefylightclient.instance.address
    );
    channels.incentivized.outbound.instance = await deployer.deploy(channels.incentivized.outbound.contract);

    // Deploy applications
    const ethApp = await deployer.deploy(
      ETHApp,
      channels.incentivized.inbound.instance.address,
      {
        inbound: channels.basic.inbound.instance.address,
        outbound: channels.basic.outbound.instance.address,
      },
      {
        inbound: channels.incentivized.inbound.instance.address,
        outbound: channels.incentivized.outbound.instance.address,
      },
    );

    const erc20App = await deployer.deploy(
      ERC20App,
      {
        inbound: channels.basic.inbound.instance.address,
        outbound: channels.basic.outbound.instance.address,
      },
      {
        inbound: channels.incentivized.inbound.instance.address,
        outbound: channels.incentivized.outbound.instance.address,
      },
    );

    const token = await deployer.deploy(TestToken, "Test Token", "TEST");

    // Deploy ERC1820 Registry for our E2E stack.
    if (network === 'e2e_test') {
      require('@openzeppelin/test-helpers/configure')({ provider: web3.currentProvider, environment: 'truffle' });
      const { singletons } = require('@openzeppelin/test-helpers');

      await singletons.ERC1820Registry(accounts[0]);
    }

    const dotApp = await deployer.deploy(
      DOTApp,
      "Snowfork DOT",
      "SnowDOT",
      channels.incentivized.outbound.instance.address,
      {
        inbound: channels.basic.inbound.instance.address,
        outbound: channels.basic.outbound.instance.address,
      },
      {
        inbound: channels.incentivized.inbound.instance.address,
        outbound: channels.incentivized.outbound.instance.address,
      },
    );

    // Account of governance contract
    // TODO: deploy the contract in this migration and use its address
    let administrator = accounts[0];

    // Post-construction initialization for Basic outbound channel
    if (!("BASIC_CHANNEL_PRINCIPAL" in process.env)) {
      throw "Missing BASIC_CHANNEL_PRINCIPAL in environment config"
    }
    const principal = process.env.BASIC_CHANNEL_PRINCIPAL
    await channels.basic.outbound.instance.initialize(
      administrator,
      principal,
      [dotApp.address, ethApp.address, erc20App.address]
    );

    // Post-construction initialization for Incentivized outbound channel
    if (!("INCENTIVIZED_CHANNEL_FEE" in process.env)) {
      throw "Missing INCENTIVIZED_CHANNEL_FEE in environment config"
    }
    const fee = process.env.INCENTIVIZED_CHANNEL_FEE
    await channels.incentivized.outbound.instance.initialize(
      administrator,
      dotApp.address,
      [dotApp.address, ethApp.address, erc20App.address]
    );
    await channels.incentivized.outbound.instance.setFee(
      fee,
      { from: administrator }
    );

    // Post-construction initialization for Incentivized inbound channel
    await channels.incentivized.inbound.instance.initialize(
      administrator,
      ethApp.address,
    );

    await token.mint("10000", {
      from: accounts[0],
    });
    await token.mint("10000", {
      from: accounts[1],
    });

  })
};
