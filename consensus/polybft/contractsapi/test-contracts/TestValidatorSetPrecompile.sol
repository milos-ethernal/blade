// SPDX-License-Identifier: MIT
// TestValidatorSetPrecompile.sol
// Contract which testst ValidatorSet precompile
pragma solidity ^0.8.0;

contract TestValidatorSetPrecompile {
    address public constant VALIDATOR_SET_PRECOMPILE = 0x0000000000000000000000000000000000002040;
    uint256 public constant VALIDATOR_SET_PRECOMPILE_GAS = 150000;

    mapping(address => bool) voteMap;
    address[] votes;

    modifier onlyValidator() {
        (bool callSuccess, bytes memory returnData) = VALIDATOR_SET_PRECOMPILE.staticcall{
            gas: VALIDATOR_SET_PRECOMPILE_GAS
        }(abi.encode(msg.sender));
        bool state = abi.decode(returnData, (bool));
        require(callSuccess && state, "validator only");
        _;
    }

    function inc() public onlyValidator {
        if (!voteMap[msg.sender]) {
            votes.push(msg.sender);
            voteMap[msg.sender] = true;
        }
    }

    function hasQuorum() public view returns (bool) {
        (bool callSuccess, bytes memory returnData) = VALIDATOR_SET_PRECOMPILE.staticcall{
            gas: VALIDATOR_SET_PRECOMPILE_GAS
        }(abi.encode(votes));
        bool state = abi.decode(returnData, (bool));
        return callSuccess && state;
    }
}
