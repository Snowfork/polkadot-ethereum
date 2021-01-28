// SPDX-License-Identifier: MIT
pragma solidity >=0.7.6;

contract VerifierWeb3 {
    address public operator;

    /**
     * @dev constructor sets the operator's address
     * @param _operator address of the contract's operator
     */
    constructor(address _operator) public {
        operator = _operator;
    }

    /**
     * @dev Verify if a hashed message was signed by the contract's operator
     * @param _hash bytes32 _hash is hashed message
     * @param _signature bytes _signature generated when operator signed the hash
     * @return bool indicating if operator is the signer
     */
    function verify(bytes32 _hash, bytes memory _signature)
        public
        view
        returns (bool)
    {
        address signer = recover(ethMessageHash(_hash), _signature);
        return operator == signer;
    }

    /**
     * @dev Recover signer address from a message using their signature
     * @param _hash bytes32 is the signed message
     * @param _signature bytes is generated by signing a hashed message
     * @return address recovered from the given hash and signature
     */
    function recover(bytes32 _hash, bytes memory _signature)
        public
        pure
        returns (address)
    {
        // Check the signature length
        if (_signature.length != 65) {
            revert("Verifier: invalid signature length");
        }

        bytes32 r;
        bytes32 s;
        uint8 v;

        // Divide the signature in r, s and v variables
        // ecrecover takes the signature parameters, and the only way to get them
        // currently is to use assembly.
        // solium-disable-next-line security/no-inline-assembly
        assembly {
            r := mload(add(_signature, 0x20))
            s := mload(add(_signature, 0x40))
            v := byte(0, mload(add(_signature, 0x60)))
        }

        // Version of signature should be 27 or 28, but 0 and 1 are also possible versions
        if (v < 27) {
            v += 27;
        }

        if (v != 27 && v != 28) {
            revert("Verifier: invalid signature 'v' value");
        }

        address signer = ecrecover(_hash, v, r, s);
        require(signer != address(0), "Verifier: invalid signature");

        return signer;
    }

    /**
     * @dev prefix a bytes32 value with "\x19Ethereum Signed Message:" and hash the result
     * @param _message bytes32 is the original, unprefixed message
     * @return bytes32 is the prefixed, hashed message
     */
    function ethMessageHash(bytes32 _message) public pure returns (bytes32) {
        return
            keccak256(
                abi.encodePacked("\x19Ethereum Signed Message:\n32", _message)
            );
    }
}
