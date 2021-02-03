const TestToken = require('../../ethereum/build/contracts/TestToken.json');
const EthClient = require('../src/ethclient').EthClient;
const SubClient = require('../src/subclient').SubClient;
const Web3 = require('web3');

const { sleep } = require('../src/helpers');
const Web3Utils = require("web3-utils");
const BigNumber = require('bignumber.js');

const { expect } = require("chai")
  .use(require("chai-as-promised"))
  .use(require("chai-bignumber")(BigNumber))

// Hardcoded based on e2e setup
const networkID = '344';
const TestTokenAddress = TestToken.networks[networkID].address;

describe('Bridge', function () {

  let ethClient;
  let subClient;

  // Address for //Alice on Substrate
  const polkadotRecipient = "0xd43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d";
  const polkadotRecipientSS58 = "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY";

  before(async function () {
    ethClient = new EthClient("ws://localhost:8545", networkID);
    subClient = new SubClient("ws://localhost:11144");
    await subClient.connect();
    await ethClient.initialize();

    this.ethAssetId = subClient.api.createType('AssetId', 'ETH');
    this.erc20AssetId = subClient.api.createType('AssetId',
      { Token: TestTokenAddress }
    );
  });

  describe('ETH App', function () {
    xit('should transfer ETH from Ethereum to Substrate', async function () {
      const amount = BigNumber(Web3.utils.toWei('0.01', "ether"));

      const account = ethClient.accounts[0];

      const beforeEthBalance = await ethClient.getEthBalance(account);
      const beforeSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.ethAssetId);

      const { gasCost } = await ethClient.lockETH(account, amount, polkadotRecipient);

      await sleep(30000);

      const afterEthBalance = await ethClient.getEthBalance(account);
      const afterSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.ethAssetId);

      expect(beforeEthBalance.minus(afterEthBalance)).to.be.bignumber.equal(amount.plus(gasCost));
      expect(afterSubBalance.minus(beforeSubBalance)).to.be.bignumber.equal(amount);

      // conservation of value
      expect(beforeEthBalance.plus(beforeSubBalance)).to.be.bignumber.equal(afterEthBalance.plus(afterSubBalance).plus(gasCost))
    });

    xit('should transfer ETH from Substrate to Ethereum', async function () {

      let amount = BigNumber('10000000000000000'); // 0.01 ETH

      const account = ethClient.accounts[0];

      let beforeEthBalance = await ethClient.getEthBalance(account);
      let beforeSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.ethAssetId);

      await subClient.burnETH(subClient.alice, account, amount.toFixed())
      await sleep(30000);

      let afterEthBalance = await ethClient.getEthBalance(account);
      let afterSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.ethAssetId);

      expect(afterEthBalance.minus(beforeEthBalance)).to.be.bignumber.equal(amount);
      expect(beforeSubBalance.minus(afterSubBalance)).to.be.bignumber.equal(amount);

      // conservation of value
      expect(beforeEthBalance.plus(beforeSubBalance)).to.be.bignumber.equal(afterEthBalance.plus(afterSubBalance))

    })
  });

  describe('ERC20 App', function () {
    it('should transfer ERC20 tokens from Ethereum to Substrate', async function () {
      let amount = BigNumber('1000');

      const account = ethClient.accounts[0];

      let beforeEthBalance = await ethClient.getErc20Balance(account);
      let beforeSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.erc20AssetId);

      await ethClient.approveERC20(account, amount);
      await ethClient.lockERC20(account, amount, polkadotRecipient);
      await sleep(30000);

      let afterEthBalance = await ethClient.getErc20Balance(account);
      let afterSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.erc20AssetId);

      expect(afterEthBalance).to.be.bignumber.equal(beforeEthBalance.minus(amount));
      expect(afterSubBalance).to.be.bignumber.equal(beforeSubBalance.plus(amount));

      // conservation of value
      expect(beforeEthBalance.plus(beforeSubBalance)).to.be.bignumber.equal(afterEthBalance.plus(afterSubBalance))
    });

    xit('should transfer ERC20 from Substrate to Ethereum', async function () {
      let amount = BigNumber('1000');

      const account = ethClient.accounts[0];

      let beforeEthBalance = await ethClient.getErc20Balance(account);
      let beforeSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.erc20AssetId);

      await subClient.burnERC20(subClient.alice, TestTokenAddress, account, amount.toFixed())
      await sleep(30000);

      let afterEthBalance = await ethClient.getErc20Balance(account);
      let afterSubBalance = await subClient.queryAccountBalance(polkadotRecipientSS58, this.erc20AssetId);

      expect(afterEthBalance.minus(beforeEthBalance)).to.be.bignumber.equal(amount);
      expect(beforeSubBalance.minus(afterSubBalance)).to.be.bignumber.equal(amount);

      // conservation of value
      expect(beforeEthBalance.plus(beforeSubBalance)).to.be.bignumber.equal(afterEthBalance.plus(afterSubBalance))
    })
  })
});
