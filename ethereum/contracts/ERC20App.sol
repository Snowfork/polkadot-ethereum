// SPDX-License-Identifier: MIT
pragma solidity >=0.7.6;
pragma experimental ABIEncoderV2;

import "@openzeppelin/contracts/math/SafeMath.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "./ScaleCodec.sol";
import "./OutboundChannel.sol";

enum ChannelId {
    Basic,
    Incentivized
}

contract ERC20App {
    using SafeMath for uint256;
    using ScaleCodec for uint256;

    mapping(address => uint256) public balances;

    mapping(ChannelId => Channel) public channels;

    event Locked(
        address token,
        address sender,
        bytes32 recipient,
        uint256 amount
    );

    event Unlocked(
        address token,
        bytes32 sender,
        address recipient,
        uint256 amount
    );

    struct OutboundPayload {
        address token;
        address sender;
        bytes32 recipient;
        uint256 amount;
    }

    struct Channel {
        address inbound;
        address outbound;
    }

    constructor(Channel memory _basic, Channel memory _incentivized) {
        Channel storage c1 = channels[ChannelId.Basic];
        c1.inbound = _basic.inbound;
        c1.outbound = _basic.outbound;

        Channel storage c2 = channels[ChannelId.Incentivized];
        c2.inbound = _incentivized.inbound;
        c2.outbound = _incentivized.outbound;
    }

    function lock(
        address _token,
        bytes32 _recipient,
        uint256 _amount,
        ChannelId _channelId
    ) public {
        require(
            IERC20(_token).transferFrom(msg.sender, address(this), _amount),
            "Contract token allowances insufficient to complete this lock request"
        );

        balances[_token] = balances[_token].add(_amount);

        emit Locked(_token, msg.sender, _recipient, _amount);

        OutboundPayload memory payload = OutboundPayload(_token, msg.sender, _recipient, _amount);

        OutboundChannel channel = OutboundChannel(channels[_channelId].outbound);
        channel.submit(encodePayload(payload));
    }

    function unlock(
        address _token,
        bytes32 _sender,
        address _recipient,
        uint256 _amount
    ) public {
        // TODO: ensure message sender is an inbound channel
        require(_amount > 0, "Must unlock a positive amount");
        require(
            _amount <= balances[_token],
            "ERC20 token balances insufficient to fulfill the unlock request"
        );

        balances[_token] = balances[_token].sub(_amount);
        require(
            IERC20(_token).transfer(_recipient, _amount),
            "ERC20 token transfer failed"
        );
        emit Unlocked(_token, _sender, _recipient, _amount);
    }

    // SCALE-encode payload
    function encodePayload(OutboundPayload memory payload) private pure returns (bytes memory) {
        return abi.encodePacked(
            payload.token,
            payload.sender,
            payload.recipient,
            payload.amount.toBytes32LE()
        );
    }
}
