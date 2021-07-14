// SPDX-License-Identifier: MIT
pragma solidity ^0.8.5;
pragma experimental ABIEncoderV2;

import "./ParachainLightClient.sol";
import "./BeefyLightClient.sol";

contract BasicInboundChannel {
    uint256 public constant MAX_GAS_PER_MESSAGE = 100000;
    uint256 public constant GAS_BUFFER = 60000;

    uint64 public nonce;

    BeefyLightClient public beefyLightClient;

    struct Message {
        address target;
        uint64 nonce; // TODO: this might cause an error, we use uint256 when encoding on Parachain
        bytes payload;
    }

    event MessageDispatched(uint64 nonce, bool result);

    constructor(BeefyLightClient _beefyLightClient) {
        nonce = 0;
        beefyLightClient = _beefyLightClient;
    }

    // TODO: add docs
    function submit(
        Message[] calldata _messages,
        ParachainLightClient.OwnParachainHeadPartial
            calldata _ownParachainHeadPartial,
        ParachainLightClient.ParachainHeadProof calldata _parachainHeadProof,
        ParachainLightClient.BeefyMMRLeafPartial calldata _beefyMMRLeafPartial,
        uint256 _beefyMMRLeafIndex,
        uint256 _beefyMMRLeafCount,
        bytes32[] calldata _beefyMMRLeafProof
    ) public {
        // Proof
        // 1. Compute our parachain's message `commitment` by ABI encoding and hashing the `_messages`
        bytes32 commitment = keccak256(abi.encode(_messages));

        ParachainLightClient.verifyCommitmentInParachain(
            commitment,
            _ownParachainHeadPartial,
            _parachainHeadProof,
            _beefyMMRLeafPartial,
            _beefyMMRLeafIndex,
            _beefyMMRLeafCount,
            _beefyMMRLeafProof,
            beefyLightClient
        );

        // Require there is enough gas to play all messages
        require(
            gasleft() >= (_messages.length * MAX_GAS_PER_MESSAGE) + GAS_BUFFER,
            "insufficient gas for delivery of all messages"
        );

        processMessages(_messages);
    }

    function processMessages(Message[] calldata _messages) internal {
        for (uint256 i = 0; i < _messages.length; i++) {
            // Check message nonce is correct and increment nonce for replay protection
            require(_messages[i].nonce == nonce + 1, "invalid nonce");

            nonce = nonce + 1;

            // Deliver the message to the target
            (bool success, ) = _messages[i].target.call{
                value: 0,
                gas: MAX_GAS_PER_MESSAGE
            }(_messages[i].payload);

            emit MessageDispatched(_messages[i].nonce, success);
        }
    }
}
