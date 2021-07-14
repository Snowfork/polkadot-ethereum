// SPDX-License-Identifier: MIT
pragma solidity >=0.7.6;
pragma experimental ABIEncoderV2;

import "@openzeppelin/contracts/access/AccessControl.sol";
import "./ChannelAccess.sol";
import "./OutboundChannel.sol";

// BasicOutboundChannel is a basic channel that just sends messages with a nonce.
contract BasicOutboundChannel is OutboundChannel, ChannelAccess, AccessControl {

    // Governance contracts will administer using this role.
    bytes32 public constant CONFIG_UPDATE_ROLE = keccak256("CONFIG_UPDATE_ROLE");

    uint64 public nonce;

    // Only messages originating from this account will
    // be allowed through the channel.
    address public principal;

    event Message(
        address source,
        uint64 nonce,
        bytes payload
    );

    constructor() {
        _setupRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }

    // Once-off post-construction call to set initial configuration.
    function initialize(
        address _configUpdater,
        address _principal,
        address[] memory defaultOperators
    )
    external {
        require(hasRole(DEFAULT_ADMIN_ROLE, msg.sender), "Caller is unauthorized");

        // Set initial configuration
        grantRole(CONFIG_UPDATE_ROLE, _configUpdater);
        principal = _principal;
        for (uint i = 0; i < defaultOperators.length; i++) {
            _authorizeDefaultOperator(defaultOperators[i]);
        }

        // drop admin privileges
        renounceRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }

    // Authorize an operator/app to submit messages for *all* users.
    function authorizeDefaultOperator(address operator) external {
        require(hasRole(CONFIG_UPDATE_ROLE, msg.sender), "Caller is unauthorized");
        _authorizeDefaultOperator(operator);
    }

    // Revoke authorization.
    function revokeDefaultOperator(address operator) external {
        require(hasRole(CONFIG_UPDATE_ROLE, msg.sender), "Caller is unauthorized");
        _revokeDefaultOperator(operator);
    }

    /**
     * @dev Sends a message across the channel
     */
    function submit(address _origin, bytes calldata _payload) external override {
        require(isOperatorFor(msg.sender, _origin), "Caller is unauthorized");
        require(_origin == principal, "Caller is unauthorized");
        nonce = nonce + 1;
        emit Message(msg.sender, nonce, _payload);
    }
}
