// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {ERC1155} from "@openzeppelin/contracts/token/ERC1155/ERC1155.sol";
import {IRaylsERC1155} from "../interfaces/IRaylsERC1155.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import "@openzeppelin/contracts/utils/math/SafeMath.sol";

contract RaylsERC1155 is IRaylsERC1155, ERC1155, AccessControl {
    using SafeMath for uint256;

    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));

    uint256 public constant DEFAULT_FUNGIBLE_DECIMALS = 18;
    // limiting max number of tokens per tokenId
    // to keep the system conpatible with ZkDvp Groth16 Snarks
    // giving at least 18 decimals + 18 digits = 36
    uint256 public constant DEFAULT_FUNGIBLE_MAX = 10 ** 36;

    uint256 constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    ///////////////////////////////////////////////
    //           Private attributes
    //////////////////////////////////////////////

    // mapping to keep track of Registered MetaTokens
    mapping(uint256 => Metadata) private _metadatas;

    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////
    constructor(string memory _uri) ERC1155(_uri) AccessControl() {
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
    }

    function supportsInterface(
        bytes4 interfaceId
    ) public view virtual override(ERC1155, AccessControl) returns (bool) {
        return super.supportsInterface(interfaceId);
    }

    ///////////////////////////////////////////////
    //              Added Functions
    //////////////////////////////////////////////

    // public function to mint one erc1155 token
    function mint(
        address _to,
        uint256 _id,
        uint256 _value,
        bytes memory _data
    ) public onlyRole(DEFAULT_OWNER_ROLE) {
        _mint(_to, _id, _value, _data);
    }

    // public function to mint multiple erc1155 token
    function mintBatch(
        address _to,
        uint256[] memory _ids,
        uint256[] memory _values,
        bytes memory _data
    ) public onlyRole(DEFAULT_OWNER_ROLE) {
        for (uint256 i = 0; i < _ids.length; i++) {
            _mint(_to, _ids[i], _values[i], _data);
        }
    }

    // Computing the tokenId based on metadata
    // if token is NORMAL or SUB_TOKEN
    //      returns stored offchainId
    // else
    //      computes unique tokenId by hashing all the attributes
    function tokenId(Metadata memory metadata) public pure returns (uint256) {
        // for NORMAL tokens the id is the pre-set offchain id.
        if (metadata.tType == TokenType.NORMAL) {
            return metadata.offchainId;
        } else {
            // adding name, symbol and offchainId to unique tokenId generation process
            bytes memory idBytes = abi.encodePacked(
                metadata.name,
                metadata.symbol,
                metadata.offchainId
            );

            // adding subTokensIds and subTokenValues to unique tokenId generation process
            for (uint i = 0; i < metadata.subTokenIds.length; i++) {
                idBytes = abi.encodePacked(
                    idBytes,
                    metadata.subTokenIds[i],
                    metadata.subTokenValues[i]
                );
            }

            // adding additional attrbutes to unique tokenId generation process
            for (uint i = 0; i < metadata.attrs.length; i++) {
                idBytes = abi.encodePacked(idBytes, metadata.attrs[i]);
            }

            // generating the uniqueId for metaToken
            return uint256(keccak256(idBytes));
        }
    }

    // returns Token's type: NORMAL, SUB_TOKEN, META_TOKEN_BOND, etc
    function tokenType(uint256 _tokenId) public view returns (TokenType) {
        Metadata memory tokenMetadata = metadata(_tokenId);
        return tokenMetadata.tType;
    }

    // returns token's state: NOT_EXISTS, REGISTERED, etc
    function tokenState(uint256 _tokenId) public view returns (TokenState) {
        Metadata memory tokenMetadata = metadata(_tokenId);
        return tokenMetadata.tState;
    }

    // simpler interface to check fungibility
    function isFungible(uint256 _tokenId) public view returns (bool) {
        Metadata memory tokenMetadata = metadata(_tokenId);

        return (tokenMetadata.tFungibility == TokenFungibility.FUNGIBLE);
    }
    // returns token's fungibility: FUNGIBLE, NON_FUNGIBLE
    function tokenFungibility(
        uint256 _tokenId
    ) public view returns (TokenFungibility) {
        Metadata memory tokenMetadata = metadata(_tokenId);
        return tokenMetadata.tFungibility;
    }

    // returns the metadata of token
    function metadata(uint256 _tokenId) public view returns (Metadata memory) {
        Metadata memory tokenMetadata = _metadatas[_tokenId];
        return tokenMetadata;
    }

    // Registers new token's metadata, given generic attributes
    function registerNewToken(
        TokenType tType,
        TokenFungibility tFungibility,
        string memory name,
        string memory symbol,
        uint256 offchainId,
        uint256 maxTotalSupply,
        uint256 decimals,
        uint256[] memory subTokenIds,
        uint256[] memory subTokenValues,
        bytes memory data, // reserved
        uint256[] memory additionalAttrs // reserved for more complex type of MetaTokens
    ) public returns (bool) {
        // Checking the length of ids and values to be the same
        if (subTokenIds.length != subTokenValues.length) {
            revert IdsValuesLengthMismatch();
        }

        Metadata memory newMetadata;

        newMetadata.tType = tType;
        newMetadata.name = name;
        newMetadata.symbol = symbol;
        newMetadata.offchainId = offchainId;

        if (tFungibility == TokenFungibility.NON_FUNGIBLE) {
            // nonfungible meta-token
            // not reading decimals and maxTotalSupply
            newMetadata.tFungibility = TokenFungibility.NON_FUNGIBLE;
            newMetadata.decimals = 0;
            newMetadata.maxTotalSupply = 1;
        } else {
            newMetadata.tFungibility = TokenFungibility.FUNGIBLE;
            if (maxTotalSupply <= 0 || maxTotalSupply > DEFAULT_FUNGIBLE_MAX) {
                revert InvalidMaxSupply();
            }
            newMetadata.maxTotalSupply = maxTotalSupply;
            newMetadata.decimals = decimals;
        }

        newMetadata.subTokenIds = new uint256[](subTokenIds.length);
        newMetadata.subTokenValues = new uint256[](subTokenValues.length);
        for (uint i = 0; i < subTokenIds.length; i++) {
            newMetadata.subTokenIds[i] = subTokenIds[i];
            newMetadata.subTokenValues[i] = subTokenValues[i];

            // TODO:: check the metaToken type to be SUB_TOKEN
            //        or state and type to be NOT_EXIST and NORMAL
            _metadatas[newMetadata.subTokenIds[i]].tType = TokenType.SUB_TOKEN;
        }

        // storing the additional attributes
        newMetadata.attrs = new uint256[](additionalAttrs.length);
        for (uint i = 0; i < additionalAttrs.length; i++) {
            newMetadata.attrs[i] = additionalAttrs[i];
        }

        // generating tokenId based on MetaToken's metadata
        uint256 newTokenId = tokenId(newMetadata);

        _metadatas[newTokenId] = newMetadata;
        // check bond state to be unregistered
        if (_metadatas[newTokenId].tState != TokenState.NOT_EXIST) {
            revert TokenAlreadyRegistered();
        }

        _metadatas[newTokenId].tState = TokenState.REGISTERED;

        emit NewTokenRegistered(newTokenId, offchainId, maxTotalSupply);

        return true;
    }

    ////////////////////////////////////////////////
    //          function overrides
    ////////////////////////////////////////////////

    function _mint(
        address _to,
        uint256 _id,
        uint256 _value,
        bytes memory _data // reserved
    ) internal override {
        if (_id == 0) {
            revert ZeroIdNotAllowed();
        }

        if (_value == 0) {
            revert ZeroValueMintNotAllowed();
        }

        // checking if the token is being minted for the first time
        // or the fungibility matches current fungibilityData
        TokenState tState = _metadatas[_id].tState;

        if (tState != TokenState.NOT_EXIST) {
            // has been registered before

            TokenFungibility tFungibility = _metadatas[_id].tFungibility;
            uint256 maxSupply = _metadatas[_id].maxTotalSupply;
            uint256 totalSupply = _metadatas[_id].totalSupply;
            // TODO:: check for overflow
            if (
                tFungibility == TokenFungibility.NON_FUNGIBLE &&
                totalSupply.add(_value) > maxSupply
            ) {
                revert ValueFungibilityInconsistency();
            }
        } else {
            // default values for decimals and maxTotalSupply
            // if minting without registering a token
            _metadatas[_id].tState = TokenState.REGISTERED;
            _metadatas[_id].tFungibility = TokenFungibility.FUNGIBLE;
            _metadatas[_id].decimals = DEFAULT_FUNGIBLE_DECIMALS;
            _metadatas[_id].maxTotalSupply = DEFAULT_FUNGIBLE_MAX;
        }

        // getting tokenType from _metadatas mapping
        TokenType tokenType_ = _metadatas[_id].tType;

        if (tokenType_ == TokenType.NORMAL) {
            // to avoid registering a metaToken over a normal token
            super._mint(_to, _id, _value, "");
            _metadatas[_id].totalSupply += _value;
        } else if (tokenType_ == TokenType.SUB_TOKEN) {
            super._mint(_to, _id, _value, "");
            _metadatas[_id].totalSupply += _value;
        } else if (tokenType_ == TokenType.META_TOKEN_BOND) {
            _mintBond(_to, _id, _value, "");
        } else if (tokenType_ == TokenType.META_TOKEN_DLC) {
            // TODO:: Add customized
            //        batchTransferFrom logic for DLC
            revert NotImplemented();
        }
    }

    function safeTransferFrom(
        address from,
        address to,
        uint256 id,
        uint256 value,
        bytes memory data
    ) public override {
        if (id == 0) {
            revert ZeroIdNotAllowed();
        }

        TokenType tokenType = _metadatas[id].tType;

        if (tokenType == TokenType.NORMAL) {
            super.safeTransferFrom(from, to, id, value, "");
        } else if (tokenType == TokenType.SUB_TOKEN) {
            super.safeTransferFrom(from, to, id, value, "");
        } else if (tokenType == TokenType.META_TOKEN_BOND) {
            super.safeTransferFrom(from, to, id, value, "");
        } else if (tokenType == TokenType.META_TOKEN_DLC) {
            // TODO:: Add customized
            //        safeTransferFrom logic for DLC

            revert NotImplemented();
        }

        // TODO:: when you wanna add new MetaToken
        //        implement new MetaToken's safeTransferFrom here
    }

    // subtoken transfer has been deactivated
    function safeBatchTransferFrom(
        address from,
        address to,
        uint256[] memory ids,
        uint256[] memory values,
        bytes memory data
    ) public override {
        for (uint256 i = 0; i < ids.length; i++) {
            TokenType tokenType = _metadatas[ids[i]].tType;
            _metadatas[ids[i]].tState = TokenState.REGISTERED;

            if (tokenType == TokenType.NORMAL) {
                super.safeTransferFrom(from, to, ids[i], values[i], "");
            } else if (tokenType == TokenType.SUB_TOKEN) {
                super.safeTransferFrom(from, to, ids[i], values[i], "");
            } else if (tokenType == TokenType.META_TOKEN_BOND) {
                super.safeTransferFrom(from, to, ids[i], values[i], "");
            } else if (tokenType == TokenType.META_TOKEN_DLC) {
                // TODO:: Add customized batch
                //        batchTransferFrom logic for DLC
                revert NotImplemented();
            }

            // TODO:: when you wanna add new MetaToken
            //        implement new MetaToken's batchTransferFrom
        }
    }

    /////////////////////////////////////////////////////
    //           Bond Internal functions
    /////////////////////////////////////////////////////

    function _mintBond(
        address to,
        uint256 onChainBondId,
        uint256 amount,
        bytes memory data
    ) internal returns (bool) {
        // check the bond exists
        if (_metadatas[onChainBondId].tState != TokenState.REGISTERED) {
            revert BondDoesNotExist();
        }

        // check not exceeding  bond.maxSupply
        if (
            _metadatas[onChainBondId].maxTotalSupply <
            amount + _metadatas[onChainBondId].totalSupply
        ) {
            revert MaxSupplyExceeded();
        }

        // computing the amount of subTokens to be burnt
        uint256[] memory tokenIds = _metadatas[onChainBondId].subTokenIds;
        uint256[] memory mulValues = new uint256[](tokenIds.length);

        for (uint256 i = 0; i < mulValues.length; i++) {
            mulValues[i] = _metadatas[onChainBondId].subTokenValues[i].mul(
                amount
            );
        }

        // burning sub-tokens
        _burnBatch(msg.sender, tokenIds, mulValues);

        // minting bond token
        super._mint(to, onChainBondId, amount, data);

        _metadatas[onChainBondId].totalSupply = _metadatas[onChainBondId]
            .totalSupply
            .add(amount);

        return true;
    }
}
