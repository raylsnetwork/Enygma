// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

// Echidna fuzzing harness for Erc20CoinVault fund-safety invariants.
//
// SCOPE: this harness only exercises deposit()/depositV2() (no ZK proof
// required — deposit only transfers real ERC20 tokens in and records a
// Poseidon commitment). withdraw()/withdrawV2()/transfer()/transferV2()
// require a valid Groth16 proof verified against the real on-chain
// verifier, which Echidna cannot forge without a mocked verifier, so
// those paths are out of scope for this campaign and are not wrapped
// here. See the security review notes for a possible follow-up harness
// that swaps in a trust-mode mock IVerifier to fuzz those paths too.

import {IPoseidonWrapper} from "../core/interfaces/IPoseidonWrapper.sol";
import {Erc20CoinVault} from "../core/contracts/vaults/Erc20CoinVault.sol";

contract MockERC20 {
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    uint256 public totalSupply;

    function mint(address to, uint256 amount) external {
        balanceOf[to] += amount;
        totalSupply += amount;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        require(balanceOf[msg.sender] >= amount, "MockERC20: balance");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        return true;
    }

    function transferFrom(
        address from,
        address to,
        uint256 amount
    ) external returns (bool) {
        require(balanceOf[from] >= amount, "MockERC20: balance");
        uint256 allowed = allowance[from][msg.sender];
        require(allowed >= amount, "MockERC20: allowance");
        if (allowed != type(uint256).max) {
            allowance[from][msg.sender] = allowed - amount;
        }
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        return true;
    }
}

// A deterministic (non-cryptographic) stand-in for the real Poseidon
// precompile wrapper. Only needs to be a pure function that never
// reverts for well-formed inputs — this harness doesn't depend on
// Poseidon's actual algebraic properties, only on deposit() completing.
contract MockPoseidonWrapper is IPoseidonWrapper {
    uint256 constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    function poseidon(
        uint256[2] memory input
    ) external pure override returns (uint256) {
        return
            uint256(keccak256(abi.encodePacked(input[0], input[1]))) %
            SNARK_SCALAR_FIELD;
    }

    function poseidon4(
        uint256[4] memory input
    ) external pure override returns (uint256) {
        return
            uint256(
                keccak256(
                    abi.encodePacked(input[0], input[1], input[2], input[3])
                )
            ) % SNARK_SCALAR_FIELD;
    }
}

contract EchidnaVaultHarness {
    Erc20CoinVault public vault;
    MockERC20 public token;
    MockPoseidonWrapper public poseidonWrapper;

    uint256 public ghost_totalDeposited;

    constructor() {
        token = new MockERC20();
        poseidonWrapper = new MockPoseidonWrapper();

        // This harness deploys the vault and passes itself as the
        // "zkDvpContractAddress", so the vault's constructor auto-grants
        // it both DEFAULT_OWNER_ROLE and DEFAULT_DVP_ROLE, letting it
        // call initializeVault() below without needing a real EnygmaDvp.
        vault = new Erc20CoinVault(address(this));
        vault.initializeVault(
            1, // vaultId
            1, // numberOfAssetIdentifiers
            address(token), // assetContractAddress
            8, // treeDepth (small so it's cheap to fill during fuzzing)
            address(poseidonWrapper), // hashContractAddress
            address(0xdead), // verifierContractAddress — never dialed on this path
            address(0xbeef) // zkAuctionContractAddress
        );
    }

    function fuzz_deposit(uint256 amount, uint256 pkSpend, uint256 salt) public {
        amount = amount % 1e24; // keep amounts in a sane range

        token.mint(address(this), amount);
        token.approve(address(vault), amount);

        uint256[] memory params = new uint256[](4);
        params[0] = amount;
        params[1] = pkSpend;
        params[2] = salt;
        params[3] = 0; // tokenId

        vault.deposit(params);
        ghost_totalDeposited += amount;
    }

    function fuzz_depositV2(
        uint256 amount,
        uint256 pkSpend,
        uint256 salt,
        bytes calldata ciphertextI,
        bytes calldata ciphertextII
    ) public {
        amount = amount % 1e24;

        token.mint(address(this), amount);
        token.approve(address(vault), amount);

        uint256[] memory params = new uint256[](4);
        params[0] = amount;
        params[1] = pkSpend;
        params[2] = salt;
        params[3] = 0;

        vault.depositV2(params, ciphertextI, ciphertextII);
        ghost_totalDeposited += amount;
    }

    // Invariant: the vault's ERC20 balance always exactly equals the sum
    // of amounts successfully deposited through it. Since this harness
    // never supplies a valid ZK proof, withdraw()/transfer() should never
    // succeed, so nothing should be able to move tokens back out — if
    // this ever breaks, tokens left the vault (or were double-counted)
    // through some path other than tracked deposits.
    function echidna_balance_matches_deposits() public view returns (bool) {
        return token.balanceOf(address(vault)) == ghost_totalDeposited;
    }
}
