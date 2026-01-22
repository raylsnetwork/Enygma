// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

interface IRaylsERC1155 {
    /////////////////////////////////////////
    //               Errors
    /////////////////////////////////////////
    error ZeroIdNotAllowed();
    error ZeroValueMintNotAllowed();
    error NotImplemented();
    error ValueFungibilityInconsistency();
    error InvalidMintData();
    error IdsValuesMismatch();
    error NotEnoughSubTokens();
    error BondDoesNotExist();
    error MaxSupplyExceeded();
    error TokenAlreadyRegistered();
    error InvalidMaxSupply();
    error IdsValuesLengthMismatch();

    /////////////////////////////////////////
    //               Enums
    /////////////////////////////////////////
    enum TokenFungibility {
        FUNGIBLE,
        NON_FUNGIBLE
    }

    enum TokenType {
        NORMAL,
        SUB_TOKEN, // has been used in a metadata-structure
        META_TOKEN_BOND,
        META_TOKEN_DLC
    }
    // TODO:: add new instrument here

    enum TokenState {
        NOT_EXIST,
        REGISTERED,
        FROZEN
    }
    // TODO:: add new state if needed

    /////////////////////////////////////////
    //     Token metadata structure
    /////////////////////////////////////////
    struct Metadata {
        TokenType tType;
        TokenState tState;
        TokenFungibility tFungibility;
        string name;
        string symbol;
        uint256 offchainId;
        uint256[] subTokenIds;
        uint256[] subTokenValues;
        uint256 totalSupply;
        uint256 maxTotalSupply;
        uint256 decimals;
        bytes data; // reserved for compatibality with Erc1155
        uint256[] attrs; // reserved for more complex instruments
    }

    /////////////////////////////////////////
    //           New Event
    /////////////////////////////////////////
    event NewTokenRegistered(
        uint256 indexed onchainId,
        uint256 indexed offchainId,
        uint256 maxTotalSupply
    );

    /////////////////////////////////////////
    //          Mint functions
    /////////////////////////////////////////

    function mint(
        address _to,
        uint256 _id,
        uint256 _value,
        bytes memory _data
    ) external;

    function mintBatch(
        address _to,
        uint256[] memory _ids,
        uint256[] memory _values,
        bytes memory _data
    ) external;

    /////////////////////////////////////////
    //      MetaToken functions
    /////////////////////////////////////////
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
    ) external returns (bool);

    // generates tokenId based on token's metadata
    function tokenId(Metadata memory) external pure returns (uint256);

    // returns token's type
    function tokenType(uint256 tokenId) external view returns (TokenType);

    // returns token's state
    function tokenState(uint256 tokenId) external view returns (TokenState);

    // simpler interface to check the fungibility of the token
    function isFungible(uint256 _tokenId) external view returns (bool);

    // returns token's fungibility
    function tokenFungibility(
        uint256 tokenId
    ) external view returns (TokenFungibility);

    // returns token's metadata
    function metadata(uint256 tokenId) external view returns (Metadata memory);
}
