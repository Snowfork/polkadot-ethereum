const {
  deployBeefyLightClient,
  mine, printBitfield, printTxPromiseGas
} = require("./helpers");
const { keccakFromHexString } = require("ethereumjs-util");

const { createBeefyValidatorFixture, createRandomPositions } = require("./beefy-helpers");
const realWorldFixture = require('./fixtures/full-flow.json');

require("chai")
  .use(require("chai-as-promised"))
  .should();

const { expect } = require("chai");

describe("Beefy Light Client Gas Usage", function () {

  it("runs new signature commitment with 200 validators", async function () {
    this.timeout(10 * 2000)
    const numberOfValidators = 200;
    const numberOfSignatures = 140;

    const fixture = await createBeefyValidatorFixture(
      numberOfValidators
    )

    this.beefyLightClient = await deployBeefyLightClient(fixture.root,
      numberOfValidators);

    const initialBitfieldPositions = await createRandomPositions(numberOfSignatures, numberOfValidators)

    const initialBitfield = await this.beefyLightClient.createInitialBitfield(
      initialBitfieldPositions, numberOfValidators
    );

    const commitmentHash = await this.beefyLightClient.createCommitmentHash(realWorldFixture.completeSubmitInput.commitment);

    console.log({ initialBitfieldPositions })
    console.log(`Initial bitfield is: ${printBitfield(initialBitfield)}`)

    const tree = fixture.validatorsMerkleTree;
    const position = initialBitfieldPositions[0]
    const leaf = tree.getHexLeaves()[position]
    const wallet = fixture.walletsByLeaf[leaf]
    const address = wallet.address
    const proof = tree.getHexProof(leaf, position)
    const signature = await wallet.signMessage(commitmentHash)

    console.log("Sending new signature commitment tx")
    const newSigTxPromise = this.beefyLightClient.newSignatureCommitment(
      commitmentHash,
      initialBitfield,
      signature,
      position,
      address,
      proof,
    )
    console.log("Sent it")
    printTxPromiseGas(newSigTxPromise)
    await newSigTxPromise.should.be.fulfilled

    const lastId = (await this.beefyLightClient.currentId()).sub(new web3.utils.BN(1));
    console.log("Onto the next one")

    await mine(45);

    const bitfield = await this.beefyLightClient.createRandomBitfield(lastId);
    console.log(`Random bitfield is: ${printBitfield(bitfield)}`)

    const validatorProof = {
      signatures: fixture200.signatures,
      positions: fixture200.positions,
      publicKeys: fixture200.validatorPublicKeys,
      publicKeyMerkleProofs: fixture200.validatorPublicKeyProofs
    }

    console.log("Sending complete signature commitment tx")
    const completeSigTxPromise = this.beefyLightClient.completeSignatureCommitment(
      lastId,
      fixture.commitment,
      validatorProof,
      fixture.beefyMMRLeaf,
      fixture.leafProof,
    )
    printTxPromiseGas(completeSigTxPromise)
    await completeSigTxPromise.should.be.fulfilled

    latestMMRRoot = await this.beefyLightClient.latestMMRRoot()
    expect(latestMMRRoot).to.eq(fixture.commitment.payload)
  });


});
