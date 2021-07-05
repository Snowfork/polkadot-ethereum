const BigNumber = web3.BigNumber;
const {
  deployBeefyLightClient,
  mine, catchRevert
} = require("./helpers");
const fixture = require('./fixtures/beefy-fixture-data.json');

require("chai")
  .use(require("chai-as-promised"))
  .use(require("chai-bignumber")(BigNumber))
  .should();

const { expect } = require("chai");

describe("Beefy Light Client", function () {

  before(async function () {
    this.timeout(10 * 1000)

    this.validatorsMerkleTree = createMerkleTree(fixture.validatorPublicKeysUncompressedScale);
    // console.log(this.validatorsMerkleTree.getHexRoot())
    this.beefyLightClient = await deployBeefyLightClient(fixture.validatorsRoot,
      fixture.validatorPublicKeysUncompressedScale.length, fixture.startingValidatorSetID);
  });

  it("encodes beefy mmr leaves correctly", async function () {
    encodedLeaf = await this.beefyLightClient.encodeMMRLeaf(fixture.beefyMMRLeaf)
    expect(encodedLeaf).to.eq(fixture.encodedBeefyLeaf)
  });

  it("hashes beefy mmr leaves correctly", async function () {
    hashedLeaf = await this.beefyLightClient.hashMMRLeaf(fixture.encodedBeefyLeaf)
    expect(hashedLeaf).to.eq(fixture.hashedBeefyLeaf)
  });

  it("runs new signature commitment and complete signature commitment correctly", async function () {
    const initialBitfield = await this.beefyLightClient.createInitialBitfield(fixture.validatorBitfield, 2);
    expect(printBitfield(initialBitfield)).to.eq('11')

    const commitmentHash = await this.beefyLightClient.createCommitmentHash(fixture.commitment);

    const tx = this.beefyLightClient.newSignatureCommitment(
      commitmentHash,
      initialBitfield,
      fixture.signatures[0],
      0,
      fixture.validatorPublicKeysUncompressedScale[0],
      fixture.validatorPublicKeyProofs[0]
    )

    await tx.should.be.fulfilled

    const lastId = (await this.beefyLightClient.currentId()).sub(new web3.utils.BN(1));

    await catchRevert(this.beefyLightClient.createRandomBitfield(lastId), 'Error: Block wait period not over');

    await mine(45);

    const bitfield = await this.beefyLightClient.createRandomBitfield(lastId);
    expect(printBitfield(bitfield)).to.eq('11')

    const validatorProof = {
      signatures: fixture.signatures,
      positions: [0, 1],
      publicKeys: fixture.validatorPublicKeysUncompressedScale,
      publicKeyMerkleProofs: fixture.validatorPublicKeyProofs
    }

    await this.beefyLightClient.completeSignatureCommitment(
      lastId,
      fixture.commitment,
      validatorProof,
      fixture.beefyMMRLeaf,
      fixture.leafProof,
    ).should.be.fulfilled

    latestMMRRoot = await this.beefyLightClient.latestMMRRoot()
    expect(latestMMRRoot).to.eq(fixture.commitment.payload)
  });


});

function printBitfield(s) {
  return parseInt(s.toString(), 10).toString(2)
}
