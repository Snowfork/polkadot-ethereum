// SPDX-License-Identifier: MIT
pragma solidity >=0.7.6;
pragma experimental ABIEncoderV2;

import "@openzeppelin/contracts/math/SafeMath.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "./Decoder.sol";
import "./OutboundChannel.sol";

contract ERC20App {
    using SafeMath for uint256;
    using Decoder for bytes;

    uint64 constant PAYLOAD_LENGTH = 104;
    string constant TARGET_APPLICATION_ID = "erc20-app";

    address public bridge;
    mapping(address => uint256) public totalTokens;
    address public basicOutboundChannelAddress;
    address public incentivizedOutboundChannelAddress;

    event Locked(
        address _sender,
        bytes32 _recipient,
        address _token,
        uint256 _amount
    );
    event Unlock(
        bytes _sender,
        address _recipient,
        address _token,
        uint256 _amount
    );

    struct ERC20LockedPayload {
        address _sender;
        bytes32 _recipient;
        address _token;
        uint256 _amount;
    }

    constructor(
        address _basicOutboundChannelAddress,
        address _incentivizedOutboundChannelAddress
    ) {
        basicOutboundChannelAddress = _basicOutboundChannelAddress;
        incentivizedOutboundChannelAddress = _incentivizedOutboundChannelAddress;
    }

    function sendERC20(
        bytes32 _recipient,
        address _tokenAddr,
        uint256 _amount,
        bool incentivized
    ) public {
        require(
            IERC20(_tokenAddr).transferFrom(msg.sender, address(this), _amount),
            "Contract token allowances insufficient to complete this lock request"
        );

        // Increment locked ERC20 token counter by this amount
        totalTokens[_tokenAddr] = totalTokens[_tokenAddr].add(_amount);

        emit Locked(msg.sender, _recipient, _tokenAddr, _amount);

        ERC20LockedPayload memory payload =
            ERC20LockedPayload(msg.sender, _recipient, _tokenAddr, _amount);
        OutboundChannel sendChannel;
        if (incentivized) {
            sendChannel = OutboundChannel(incentivizedOutboundChannelAddress);
        } else {
            sendChannel = OutboundChannel(basicOutboundChannelAddress);
        }
        sendChannel.submit(abi.encode(payload));
    }

    function sendTokens(
        address _recipient,
        address _token,
        uint256 _amount
    ) internal {
        require(_amount > 0, "Must unlock a positive amount");
        require(
            _amount <= totalTokens[_token],
            "ERC20 token balances insufficient to fulfill the unlock request"
        );

        totalTokens[_token] = totalTokens[_token].sub(_amount);
        require(
            IERC20(_token).transfer(_recipient, _amount),
            "ERC20 token transfer failed"
        );
    }
}
